package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SabhyaAggarwal/gritloom/internal/auth"
)

var (
	projectKeyPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	eventIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
	sourcePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,39}$`)
	commitPattern     = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	colorPattern      = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

var allowedKinds = map[string]bool{
	"focus": true, "edit": true, "save": true, "ai_start": true,
	"ai_end": true, "ai_tool": true, "build": true, "test": true,
	"debug": true, "commit": true,
}

var projectColors = []string{"#287a5a", "#d86745", "#3972a8", "#aa6a25", "#735f9e", "#27777b"}

func cleanText(value string, maximum int, allowNewlines bool) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maximum || !utf8.ValidString(value) {
		return "", fmt.Errorf("text exceeds %d bytes or is not valid UTF-8", maximum)
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && (allowNewlines && r != '\n' && r != '\r' && r != '\t' || !allowNewlines)
	}) >= 0 {
		return "", fmt.Errorf("text contains control characters")
	}
	return value, nil
}

func cleanRepository(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 500 {
		return "", fmt.Errorf("repository URL is too long")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("repository URL must be an http(s) URL without credentials, query, or fragment")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String(), nil
}

func validateEvent(event Event, now time.Time) error {
	if !eventIDPattern.MatchString(event.ID) {
		return fmt.Errorf("event id must be 16-64 URL-safe characters")
	}
	if !projectKeyPattern.MatchString(event.ProjectPath) {
		return fmt.Errorf("project_path must be a lowercase SHA-256 project hash")
	}
	name, err := cleanText(event.ProjectName, 80, false)
	if err != nil || name == "" {
		return fmt.Errorf("project_name is required and must be at most 80 safe characters")
	}
	if _, err := cleanRepository(event.RepositoryURL); err != nil {
		return err
	}
	if _, err := cleanText(event.Branch, 200, false); err != nil {
		return fmt.Errorf("invalid branch")
	}
	if !sourcePattern.MatchString(event.Source) || !allowedKinds[event.Kind] {
		return fmt.Errorf("unknown event source or kind")
	}
	if event.File != "" {
		if len(event.File) > 300 || strings.Contains(event.File, "\\") || strings.HasPrefix(event.File, "/") || path.Clean(event.File) != event.File || event.File == "." || strings.HasPrefix(event.File, "../") || !utf8.ValidString(event.File) || strings.IndexFunc(event.File, unicode.IsControl) >= 0 {
			return fmt.Errorf("file must be a clean repository-relative slash-separated path")
		}
	}
	if event.Additions < 0 || event.Deletions < 0 || event.Additions > 10_000_000 || event.Deletions > 10_000_000 {
		return fmt.Errorf("invalid change counts")
	}
	if event.Model != "" {
		if _, err := cleanText(event.Model, 100, false); err != nil {
			return fmt.Errorf("invalid model name")
		}
	}
	if event.CommitHash != "" && !commitPattern.MatchString(event.CommitHash) {
		return fmt.Errorf("invalid commit hash")
	}
	if event.OccurredAt.IsZero() || event.OccurredAt.After(now.Add(10*time.Minute)) || event.OccurredAt.Before(now.AddDate(-2, 0, 0)) {
		return fmt.Errorf("event timestamp is outside the accepted range")
	}
	return nil
}

func eventFingerprint(userID, projectID string, event Event) string {
	canonical, _ := json.Marshal(event)
	digest := sha256.Sum256([]byte(userID + "\n" + projectID + "\n" + string(canonical)))
	return hex.EncodeToString(digest[:])
}

func projectID(userID, key string) string {
	digest := sha256.Sum256([]byte(userID + "\n" + key))
	return hex.EncodeToString(digest[:16])
}

func slugify(name string) string {
	var builder strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			separator = false
			builder.WriteRune(r)
		} else {
			separator = true
		}
		if builder.Len() >= 48 {
			break
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "project"
	}
	return slug
}

func (s *Server) authenticateDevice(r *http.Request) (string, string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || len(header) != 7+43 {
		return "", "", false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	var userID, deviceID string
	err := s.db.QueryRowContext(r.Context(), `SELECT user_id, id FROM devices WHERE token_hash = ? AND revoked_at IS NULL`, auth.TokenHash(token)).Scan(&userID, &deviceID)
	return userID, deviceID, err == nil
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	userID, deviceID, ok := s.authenticateDevice(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid device token")
		return
	}
	if !s.limiter.allow("ingest:"+deviceID, 120, time.Minute, s.now()) {
		writeError(w, http.StatusTooManyRequests, "device upload rate limit exceeded")
		return
	}
	var input ingestRequest
	if err := decodeJSON(w, r, 2*1024*1024, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(input.Events) == 0 || len(input.Events) > 500 {
		writeError(w, http.StatusBadRequest, "events must contain between 1 and 500 entries")
		return
	}
	if input.DeviceID == "" || len(input.DeviceID) > 128 || strings.IndexFunc(input.DeviceID, unicode.IsControl) >= 0 {
		writeError(w, http.StatusBadRequest, "invalid device_id")
		return
	}
	now := s.now().UTC()
	for i := range input.Events {
		if err := validateEvent(input.Events[i], now); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("event %d: %s", i, err.Error()))
			return
		}
	}
	var count int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM events WHERE user_id = ? AND occurred_at > ?`, userID, now.Add(-24*time.Hour).UnixMilli()).Scan(&count); err != nil || count+len(input.Events) > 50000 {
		writeError(w, http.StatusTooManyRequests, "daily event quota exceeded")
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store events")
		return
	}
	defer tx.Rollback()
	accepted, duplicates := 0, 0
	for _, event := range input.Events {
		projectID := projectID(userID, event.ProjectPath)
		colorIndex := int(event.ProjectPath[0]) % len(projectColors)
		_, err := tx.ExecContext(r.Context(), `INSERT INTO projects (id, user_id, project_key, name, slug, repository_url, color, created_at, last_active_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, project_key) DO UPDATE SET last_active_at = MAX(last_active_at, excluded.last_active_at)`, projectID, userID, event.ProjectPath, strings.TrimSpace(event.ProjectName), slugify(event.ProjectName), event.RepositoryURL, projectColors[colorIndex], now.UnixMilli(), event.OccurredAt.UnixMilli())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not update project")
			return
		}
		fingerprint := eventFingerprint(userID, projectID, event)
		result, err := tx.ExecContext(r.Context(), `INSERT INTO events (id, user_id, device_id, project_id, occurred_at, branch, source, kind, file, additions, deletions, exit_code, ai, background, model, commit_hash, fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, id) DO NOTHING`, event.ID, userID, deviceID, projectID, event.OccurredAt.UnixMilli(), event.Branch, event.Source, event.Kind, event.File, event.Additions, event.Deletions, event.ExitCode, event.AI, event.Background, event.Model, event.CommitHash, fingerprint)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not store events")
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not confirm events")
			return
		}
		if rows == 1 {
			accepted++
			continue
		}
		var storedFingerprint string
		if err := tx.QueryRowContext(r.Context(), `SELECT fingerprint FROM events WHERE user_id = ? AND id = ?`, userID, event.ID).Scan(&storedFingerprint); err != nil {
			writeError(w, http.StatusInternalServerError, "could not verify duplicate event")
			return
		}
		if subtle.ConstantTimeCompare([]byte(storedFingerprint), []byte(fingerprint)) != 1 {
			writeError(w, http.StatusConflict, "event id was already used with different contents")
			return
		}
		duplicates++
	}
	if accepted > 0 {
		if err := rebuildSessions(r.Context(), tx, userID); err != nil {
			log.Printf("rebuild sessions for user %s: %v", userID, err)
			writeError(w, http.StatusInternalServerError, "could not rebuild activity timeline")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE devices SET last_seen_at = ? WHERE id = ?`, now.UnixMilli(), deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update device")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not commit events")
		return
	}
	if accepted > 0 {
		s.publishDashboard(userID)
	}
	writeJSON(w, http.StatusOK, map[string]int{"accepted": accepted, "duplicates": duplicates})
}

func rebuildSessions(ctx context.Context, tx *sql.Tx, userID string) error {
	events, err := loadEvents(tx, userID)
	if err != nil {
		return err
	}
	sessions := buildSessions(userID, events)
	if _, err := tx.ExecContext(ctx, `DELETE FROM work_sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, session := range sessions {
		evidence, err := json.Marshal(session.Evidence)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO work_sessions (id, user_id, project_id, start_at, end_at, seconds, kind, source, verification, edit_count, ai_count, test_count, failed_test_count, commit_count, evidence_hash, evidence_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, session.ID, userID, session.ProjectID, session.StartAt.UnixMilli(), session.EndAt.UnixMilli(), session.Seconds, session.Kind, session.Source, session.Verification, session.EditCount, session.AICount, session.TestCount, session.FailedTestCount, session.CommitCount, session.EvidenceHash, string(evidence))
		if err != nil {
			return err
		}
	}
	return nil
}
