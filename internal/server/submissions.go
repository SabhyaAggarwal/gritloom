package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/auth"
)

type submissionRow struct {
	Submission
	CreatedMilliseconds int64
}

func scanSubmission(row userRow) (submissionRow, error) {
	var submission submissionRow
	err := row.Scan(&submission.ID, &submission.Author, &submission.CreatedMilliseconds, &submission.Note, &submission.Seconds, &submission.SessionCount, &submission.Status, &submission.ManifestHash)
	submission.CreatedAt = time.UnixMilli(submission.CreatedMilliseconds).UTC()
	submission.Sessions = make([]Session, 0)
	return submission, err
}

const submissionColumns = `s.id, u.username, s.created_at, s.note, s.seconds, s.session_count, s.status, s.manifest_hash`

func loadSubmissionSnapshots(ctx context.Context, querier contextQuerier, submissionID string) ([]Session, error) {
	rows, err := querier.QueryContext(ctx, `SELECT snapshot FROM submission_sessions WHERE submission_id = ? ORDER BY session_id`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make([]Session, 0)
	for rows.Next() {
		var data []byte
		var session Session
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &session); err != nil {
			return nil, fmt.Errorf("decode submission snapshot: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartAt.Equal(sessions[j].StartAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].StartAt.Before(sessions[j].StartAt)
	})
	return sessions, nil
}

func (s *Server) handleSubmissions(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.QueryContext(r.Context(), `SELECT `+submissionColumns+` FROM submissions s JOIN users u ON u.id = s.user_id WHERE s.user_id = ? ORDER BY s.created_at DESC LIMIT 100`, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load submissions")
			return
		}
		submissions := make([]Submission, 0)
		for rows.Next() {
			submission, err := scanSubmission(rows)
			if err != nil {
				rows.Close()
				writeError(w, http.StatusInternalServerError, "could not load submissions")
				return
			}
			submission.Sessions, err = loadSubmissionSnapshots(r.Context(), s.db, submission.ID)
			if err != nil {
				rows.Close()
				writeError(w, http.StatusInternalServerError, "could not load submission evidence")
				return
			}
			submissions = append(submissions, submission.Submission)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "could not load submissions")
			return
		}
		rows.Close()
		writeJSON(w, http.StatusOK, submissions)
	case http.MethodPost:
		if !s.sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		var input struct {
			SessionIDs []string `json:"session_ids"`
			Note       string   `json:"note"`
		}
		if err := decodeJSON(w, r, 64*1024, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(input.SessionIDs) == 0 || len(input.SessionIDs) > 100 {
			writeError(w, http.StatusBadRequest, "select between 1 and 100 sessions")
			return
		}
		note, err := cleanText(input.Note, 2000, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, "note must be valid text of at most 2000 bytes")
			return
		}
		selected := make(map[string]struct{}, len(input.SessionIDs))
		for _, id := range input.SessionIDs {
			if len(id) != 32 {
				writeError(w, http.StatusBadRequest, "invalid session id")
				return
			}
			selected[id] = struct{}{}
		}
		tx, err := s.db.BeginTx(r.Context(), nil)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create submission")
			return
		}
		defer tx.Rollback()
		sessions, err := loadSessions(r.Context(), tx, user.ID, true, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load selected evidence")
			return
		}
		chosen := make([]Session, 0, len(selected))
		var seconds int64
		for _, session := range sessions {
			if _, ok := selected[session.ID]; !ok {
				continue
			}
			if session.Kind != "engaged" {
				writeError(w, http.StatusBadRequest, "background AI sessions cannot be submitted as engaged time")
				return
			}
			delete(selected, session.ID)
			chosen = append(chosen, session)
			seconds += session.Seconds
		}
		if len(selected) != 0 || len(chosen) == 0 || seconds <= 0 {
			writeError(w, http.StatusBadRequest, "one or more sessions are unavailable")
			return
		}
		sort.Slice(chosen, func(i, j int) bool {
			if chosen[i].StartAt.Equal(chosen[j].StartAt) {
				return chosen[i].ID < chosen[j].ID
			}
			return chosen[i].StartAt.Before(chosen[j].StartAt)
		})
		id, err := randomID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create submission")
			return
		}
		token, err := auth.NewToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create reviewer link")
			return
		}
		now := s.now().UTC().Truncate(time.Millisecond)
		manifest, err := manifestJSON(Submission{ID: id, Author: user.Username, CreatedAt: now, Note: note, Sessions: chosen})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create evidence manifest")
			return
		}
		manifestDigest := sha256.Sum256(manifest)
		manifestHash := hex.EncodeToString(manifestDigest[:])
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO submissions (id, token_hash, user_id, note, seconds, session_count, manifest_hash, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, auth.TokenHash(token), user.ID, note, seconds, len(chosen), manifestHash, now.UnixMilli()); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create submission")
			return
		}
		for _, session := range chosen {
			snapshot, err := json.Marshal(session)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not snapshot evidence")
				return
			}
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO submission_sessions (submission_id, session_id, snapshot) VALUES (?, ?, ?)`, id, session.ID, string(snapshot)); err != nil {
				writeError(w, http.StatusInternalServerError, "could not snapshot evidence")
				return
			}
		}
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not commit submission")
			return
		}
		writeJSON(w, http.StatusCreated, Submission{ID: id, Author: user.Username, Token: token, CreatedAt: now, Note: note, Seconds: seconds, SessionCount: len(chosen), Status: "pending", ManifestHash: manifestHash, Sessions: chosen})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/submissions/")
	rawManifest := strings.HasSuffix(path, "/manifest")
	token := strings.TrimSuffix(path, "/manifest")
	if token == "" || len(token) != 43 || strings.Contains(token, "/") {
		writeError(w, http.StatusNotFound, "submission not found")
		return
	}
	row, err := scanSubmission(s.db.QueryRowContext(r.Context(), `SELECT `+submissionColumns+` FROM submissions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, auth.TokenHash(token)))
	if err != nil {
		writeError(w, http.StatusNotFound, "submission not found")
		return
	}
	if row.Status == "revoked" {
		writeError(w, http.StatusGone, "this reviewer link was revoked")
		return
	}
	row.Sessions, err = loadSubmissionSnapshots(r.Context(), s.db, row.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load submission evidence")
		return
	}
	if rawManifest {
		manifest, err := manifestJSON(row.Submission)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not encode submission manifest")
			return
		}
		digest := sha256.Sum256(manifest)
		hash := hex.EncodeToString(digest[:])
		if hash != row.ManifestHash {
			writeError(w, http.StatusInternalServerError, "submission manifest integrity check failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Gritloom-Manifest-SHA256", hash)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(manifest); err != nil {
			log.Printf("write submission manifest: %v", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, row.Submission)
}

func manifestJSON(submission Submission) ([]byte, error) {
	return json.Marshal(struct {
		Version      int       `json:"version"`
		SubmissionID string    `json:"submission_id"`
		Author       string    `json:"author"`
		CreatedAt    time.Time `json:"created_at"`
		Note         string    `json:"note"`
		Sessions     []Session `json:"sessions"`
	}{1, submission.ID, submission.Author, submission.CreatedAt, submission.Note, submission.Sessions})
}

func verifyManifest(submission Submission) (string, error) {
	manifest, err := manifestJSON(submission)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(manifest)
	return hex.EncodeToString(digest[:]), nil
}
