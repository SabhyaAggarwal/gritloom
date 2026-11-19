package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/auth"
)

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		projects, err := loadProjects(r.Context(), s.db, user.ID, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load projects")
			return
		}
		writeJSON(w, http.StatusOK, projects)
	case http.MethodPost:
		if !s.sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		var input struct {
			Name          string `json:"name"`
			RepositoryURL string `json:"repository_url"`
			Description   string `json:"description"`
			Color         string `json:"color"`
		}
		if err := decodeJSON(w, r, 32*1024, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		name, err := cleanText(input.Name, 80, false)
		if err != nil || name == "" {
			writeError(w, http.StatusBadRequest, "project name is required and must be at most 80 safe characters")
			return
		}
		repository, err := cleanRepository(input.RepositoryURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		description, err := cleanText(input.Description, 500, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, "description must be at most 500 safe characters")
			return
		}
		if input.Color == "" {
			input.Color = projectColors[0]
		}
		if !colorPattern.MatchString(input.Color) {
			writeError(w, http.StatusBadRequest, "color must be a six-digit hex color")
			return
		}
		id, err := randomID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create project")
			return
		}
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create project")
			return
		}
		key := hex.EncodeToString(keyBytes)
		now := s.now().UTC()
		_, err = s.db.ExecContext(r.Context(), `INSERT INTO projects (id, user_id, project_key, name, slug, repository_url, description, color, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, user.ID, key, name, slugify(name), repository, description, input.Color, now.UnixMilli())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create project")
			return
		}
		projects, err := loadProjects(r.Context(), s.db, user.ID, false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "project created; reload to view it")
			return
		}
		for _, project := range projects {
			if project.ID == id {
				s.publishDashboard(user.ID)
				writeJSON(w, http.StatusCreated, project)
				return
			}
		}
		writeError(w, http.StatusInternalServerError, "project created; reload to view it")
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		methodNotAllowed(w, http.MethodPatch)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/")
	if id == "" || strings.Contains(id, "/") || len(id) != 32 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var input struct {
		IsPublic *bool `json:"is_public"`
	}
	if err := decodeJSON(w, r, 4*1024, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.IsPublic == nil {
		writeError(w, http.StatusBadRequest, "is_public is required")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `UPDATE projects SET is_public = ? WHERE id = ? AND user_id = ?`, *input.IsPublic, id, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update project visibility")
		return
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	projects, err := loadProjects(r.Context(), s.db, user.ID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "project updated; reload to view it")
		return
	}
	for _, project := range projects {
		if project.ID == id {
			writeJSON(w, http.StatusOK, project)
			s.publishDashboard(user.ID)
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "project updated; reload to view it")
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.QueryContext(r.Context(), `SELECT id, name, created_at, last_seen_at FROM devices WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at DESC`, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load devices")
			return
		}
		defer rows.Close()
		devices := make([]Device, 0)
		for rows.Next() {
			var device Device
			var created int64
			var lastSeen sql.NullInt64
			if err := rows.Scan(&device.ID, &device.Name, &created, &lastSeen); err != nil {
				writeError(w, http.StatusInternalServerError, "could not load devices")
				return
			}
			device.CreatedAt = time.UnixMilli(created).UTC()
			if lastSeen.Valid {
				value := time.UnixMilli(lastSeen.Int64).UTC()
				device.LastSeen = &value
			}
			devices = append(devices, device)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not load devices")
			return
		}
		writeJSON(w, http.StatusOK, devices)
	case http.MethodPost:
		if !s.sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(w, r, 16*1024, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		name, err := cleanText(input.Name, 80, false)
		if err != nil || name == "" {
			writeError(w, http.StatusBadRequest, "device name is required and must be at most 80 safe characters")
			return
		}
		var active int
		if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM devices WHERE user_id = ? AND revoked_at IS NULL`, user.ID).Scan(&active); err != nil || active >= 20 {
			writeError(w, http.StatusBadRequest, "device limit reached; revoke an unused device first")
			return
		}
		id, err := randomID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create device")
			return
		}
		token, err := auth.NewToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create device")
			return
		}
		now := s.now().UTC()
		if _, err := s.db.ExecContext(r.Context(), `INSERT INTO devices (id, user_id, name, token_hash, created_at) VALUES (?, ?, ?, ?, ?)`, id, user.ID, name, auth.TokenHash(token), now.UnixMilli()); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create device")
			return
		}
		writeJSON(w, http.StatusCreated, Device{ID: id, Name: name, Token: token, CreatedAt: now})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok || !s.sameOrigin(r) {
		if ok {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
		}
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	if id == "" || strings.Contains(id, "/") || len(id) != 32 {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `UPDATE devices SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, s.now().UnixMilli(), id, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not revoke device")
		return
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
