package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SabhyaAggarwal/gritloom/internal/auth"
)

const sessionLifetime = 30 * 24 * time.Hour

func (s *Server) sessionCookieName() string {
	if s.config.SecureCookies {
		return "__Host-gritloom_session"
	}
	return "gritloom_session"
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: s.sessionCookieName(), Value: token, Path: "/", HttpOnly: true,
		Secure: s.config.SecureCookies, SameSite: http.SameSiteLaxMode,
		Expires: expires, MaxAge: int(sessionLifetime.Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: s.sessionCookieName(), Value: "", Path: "/", HttpOnly: true,
		Secure: s.config.SecureCookies, SameSite: http.SameSiteLaxMode,
		Expires: time.Unix(1, 0), MaxAge: -1,
	})
}

type userRow interface {
	Scan(...any) error
}

func scanUser(row userRow) (User, error) {
	var user User
	var isPublic int
	var createdAt int64
	err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Bio, &isPublic, &user.WeeklyGoalSeconds, &user.TimeZone, &createdAt)
	user.IsPublic = isPublic != 0
	user.CreatedAt = time.UnixMilli(createdAt).UTC()
	return user, err
}

const userColumns = `id, username, display_name, bio, is_public, weekly_goal_seconds, timezone, created_at`

func (s *Server) userByID(ctx context.Context, userID string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, userID))
}

func (s *Server) currentUser(r *http.Request) (User, error) {
	cookie, err := r.Cookie(s.sessionCookieName())
	if err != nil || cookie.Value == "" || len(cookie.Value) > 128 {
		return User{}, sql.ErrNoRows
	}
	return scanUser(s.db.QueryRowContext(r.Context(), `SELECT u.id, u.username, u.display_name, u.bio, u.is_public, u.weekly_goal_seconds, u.timezone, u.created_at FROM users u JOIN auth_sessions s ON s.user_id = u.id WHERE s.token_hash = ? AND s.expires_at > ?`, auth.TokenHash(cookie.Value), s.now().UnixMilli()))
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "sign in required")
		return User{}, false
	}
	return user, true
}

func validDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 || !utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n") {
		return "", fmt.Errorf("display name must be 1-80 valid characters without line breaks")
	}
	return name, nil
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	ip := clientIP(r, s.config.TrustProxy)
	if !s.limiter.allow("register:"+ip, 5, time.Hour, s.now()) {
		writeError(w, http.StatusTooManyRequests, "too many registrations; try again later")
		return
	}
	var input struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(w, r, 16*1024, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	username, err := auth.ValidateUsername(input.Username)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	displayName, err := validDisplayName(input.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	token, err := auth.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO users (id, username, display_name, password_hash, created_at) VALUES (?, ?, ?, ?, ?)`, userID, username, displayName, passwordHash, now.UnixMilli())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "username is already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO auth_sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`, auth.TokenHash(token), userID, now.UnixMilli(), now.Add(sessionLifetime).UnixMilli())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	user, err := s.userByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "account created; please sign in")
		return
	}
	s.setSessionCookie(w, token, now.Add(sessionLifetime))
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, 16*1024, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ip := clientIP(r, s.config.TrustProxy)
	key := ip + ":" + strings.ToLower(strings.TrimSpace(input.Username))
	if !s.limiter.allow("login:"+key, 10, 15*time.Minute, s.now()) || !s.limiter.allow("login-ip:"+ip, 50, 15*time.Minute, s.now()) {
		writeError(w, http.StatusTooManyRequests, "too many sign-in attempts; try again later")
		return
	}
	var user User
	var passwordHash string
	var isPublic int
	var createdAt int64
	err := s.db.QueryRowContext(r.Context(), `SELECT id, username, display_name, bio, is_public, weekly_goal_seconds, timezone, created_at, password_hash FROM users WHERE username = ? COLLATE NOCASE`, strings.TrimSpace(input.Username)).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Bio, &isPublic, &user.WeeklyGoalSeconds, &user.TimeZone, &createdAt, &passwordHash)
	if err != nil {
		auth.CheckPassword(s.dummyHash, input.Password)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if !auth.CheckPassword(passwordHash, input.Password) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	user.IsPublic = isPublic != 0
	user.CreatedAt = time.UnixMilli(createdAt).UTC()
	token, err := auth.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	now := s.now().UTC()
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE expires_at <= ?`, now.UnixMilli()); err == nil {
		_, err = s.db.ExecContext(r.Context(), `INSERT INTO auth_sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`, auth.TokenHash(token), user.ID, now.UnixMilli(), now.Add(sessionLifetime).UnixMilli())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	s.setSessionCookie(w, token, now.Add(sessionLifetime))
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	if cookie, err := r.Cookie(s.sessionCookieName()); err == nil && len(cookie.Value) <= 128 {
		if _, err := s.db.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE token_hash = ?`, auth.TokenHash(cookie.Value)); err != nil {
			writeError(w, http.StatusInternalServerError, "could not sign out")
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, user)
	case http.MethodPatch:
		if !s.sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		var input struct {
			DisplayName       *string `json:"display_name"`
			Bio               *string `json:"bio"`
			IsPublic          *bool   `json:"is_public"`
			WeeklyGoalSeconds *int64  `json:"weekly_goal_seconds"`
			TimeZone          *string `json:"timezone"`
		}
		if err := decodeJSON(w, r, 16*1024, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if input.DisplayName != nil {
			name, err := validDisplayName(*input.DisplayName)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			user.DisplayName = name
		}
		if input.Bio != nil {
			bio := strings.TrimSpace(*input.Bio)
			if len(bio) > 500 || !utf8.ValidString(bio) || strings.ContainsRune(bio, '\x00') {
				writeError(w, http.StatusBadRequest, "bio must be valid text of at most 500 bytes")
				return
			}
			user.Bio = bio
		}
		if input.IsPublic != nil {
			user.IsPublic = *input.IsPublic
		}
		if input.WeeklyGoalSeconds != nil {
			if *input.WeeklyGoalSeconds < 1800 || *input.WeeklyGoalSeconds > 604800 || *input.WeeklyGoalSeconds%900 != 0 {
				writeError(w, http.StatusBadRequest, "weekly goal must be between 30 minutes and 168 hours in 15-minute steps")
				return
			}
			user.WeeklyGoalSeconds = *input.WeeklyGoalSeconds
		}
		if input.TimeZone != nil {
			if len(*input.TimeZone) > 100 {
				writeError(w, http.StatusBadRequest, "invalid timezone")
				return
			}
			if _, err := time.LoadLocation(*input.TimeZone); err != nil {
				writeError(w, http.StatusBadRequest, "invalid IANA timezone")
				return
			}
			user.TimeZone = *input.TimeZone
		}
		_, err := s.db.ExecContext(r.Context(), `UPDATE users SET display_name = ?, bio = ?, is_public = ?, weekly_goal_seconds = ?, timezone = ? WHERE id = ?`, user.DisplayName, user.Bio, user.IsPublic, user.WeeklyGoalSeconds, user.TimeZone, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save settings")
			return
		}
		writeJSON(w, http.StatusOK, user)
		s.publishDashboard(user.ID)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
