package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

type contextQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type timeInterval struct {
	start time.Time
	end   time.Time
}

func loadEngagedAISeconds(ctx context.Context, querier contextQuerier, userID string, start, end time.Time, publicOnly bool) (int64, error) {
	query := `SELECT e.project_id, e.occurred_at FROM events e`
	if publicOnly {
		query += ` JOIN projects p ON p.id = e.project_id AND p.is_public = 1`
	}
	query += ` WHERE e.user_id = ? AND e.ai = 1 AND e.background = 0 AND e.kind IN ('ai_start', 'ai_end', 'ai_tool') ORDER BY e.occurred_at, e.id`
	rows, err := querier.QueryContext(ctx, query, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	previous := make(map[string]time.Time)
	intervals := make([]timeInterval, 0)
	for rows.Next() {
		var projectID string
		var milliseconds int64
		if err := rows.Scan(&projectID, &milliseconds); err != nil {
			return 0, err
		}
		occurredAt := time.UnixMilli(milliseconds).UTC()
		if prior, ok := previous[projectID]; ok {
			if gap := occurredAt.Sub(prior); gap > 0 && gap <= engagedMaxGap {
				intervals = append(intervals, timeInterval{start: prior, end: occurredAt})
			}
		}
		previous[projectID] = occurredAt
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start.Before(intervals[j].start) })
	var total int64
	var merged timeInterval
	for i, interval := range intervals {
		if i > 0 && !interval.start.After(merged.end) {
			if interval.end.After(merged.end) {
				merged.end = interval.end
			}
			continue
		}
		if i > 0 {
			total += overlapSeconds(Session{StartAt: merged.start, EndAt: merged.end}, start, end)
		}
		merged = interval
	}
	if len(intervals) > 0 {
		total += overlapSeconds(Session{StartAt: merged.start, EndAt: merged.end}, start, end)
	}
	return total, nil
}

func loadSessions(ctx context.Context, querier contextQuerier, userID string, includeEvidence, publicOnly bool) ([]Session, error) {
	rows, err := querier.QueryContext(ctx, `SELECT s.id, s.project_id, p.name, p.color, s.start_at, s.end_at, s.seconds, s.kind, s.source, s.verification, s.edit_count, s.ai_count, s.test_count, s.failed_test_count, s.commit_count, s.evidence_hash, s.evidence_json, p.is_public FROM work_sessions s JOIN projects p ON p.id = s.project_id WHERE s.user_id = ? ORDER BY s.start_at, s.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session
		var start, end int64
		var evidence []byte
		var public int
		if err := rows.Scan(&session.ID, &session.ProjectID, &session.ProjectName, &session.ProjectColor, &start, &end, &session.Seconds, &session.Kind, &session.Source, &session.Verification, &session.EditCount, &session.AICount, &session.TestCount, &session.FailedTestCount, &session.CommitCount, &session.EvidenceHash, &evidence, &public); err != nil {
			return nil, err
		}
		if publicOnly && public == 0 {
			continue
		}
		session.StartAt = time.UnixMilli(start).UTC()
		session.EndAt = time.UnixMilli(end).UTC()
		if includeEvidence {
			session.Evidence = make([]EvidenceEvent, 0)
			if err := json.Unmarshal(evidence, &session.Evidence); err != nil {
				return nil, fmt.Errorf("decode stored evidence: %w", err)
			}
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func loadProjects(ctx context.Context, querier contextQuerier, userID string, publicOnly bool) ([]Project, error) {
	query := `SELECT id, name, slug, repository_url, description, color, language, last_active_at, is_public FROM projects WHERE user_id = ?`
	if publicOnly {
		query += ` AND is_public = 1`
	}
	query += ` ORDER BY COALESCE(last_active_at, created_at) DESC, name`
	rows, err := querier.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		var lastActive sql.NullInt64
		var public int
		if err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.RepositoryURL, &project.Description, &project.Color, &project.Language, &lastActive, &public); err != nil {
			return nil, err
		}
		project.IsPublic = public != 0
		if lastActive.Valid {
			value := time.UnixMilli(lastActive.Int64).UTC()
			project.LastActiveAt = &value
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func overlapSeconds(session Session, start, end time.Time) int64 {
	startMs, endMs := start.UnixMilli(), end.UnixMilli()
	overlapStart, overlapEnd := session.StartAt.UnixMilli(), session.EndAt.UnixMilli()
	if overlapStart < startMs {
		overlapStart = startMs
	}
	if overlapEnd > endMs {
		overlapEnd = endMs
	}
	if overlapEnd <= overlapStart {
		return 0
	}
	return (overlapEnd - overlapStart) / 1000
}

func dayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func (s *Server) dashboard(ctx context.Context, user User, publicOnly bool) (Dashboard, error) {
	location, err := time.LoadLocation(user.TimeZone)
	if err != nil {
		location = time.UTC
	}
	projects, err := loadProjects(ctx, s.db, user.ID, publicOnly)
	if err != nil {
		return Dashboard{}, err
	}
	sessions, err := loadSessions(ctx, s.db, user.ID, false, publicOnly)
	if err != nil {
		return Dashboard{}, err
	}
	now := s.now().UTC()
	todayStart := dayStart(now, location)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	weekdayOffset := (int(todayStart.Weekday()) + 6) % 7
	weekStart := todayStart.AddDate(0, 0, -weekdayOffset)
	stats := Stats{WeeklyGoalSeconds: user.WeeklyGoalSeconds}
	stats.AISeconds, err = loadEngagedAISeconds(ctx, s.db, user.ID, todayStart, tomorrowStart, publicOnly)
	if err != nil {
		return Dashboard{}, err
	}
	projectTotals := make(map[string]*Project, len(projects))
	for i := range projects {
		projectTotals[projects[i].ID] = &projects[i]
	}
	secondsByDate := make(map[string]int64)
	aiByDate := make(map[string]int64)
	live := false
	for _, session := range sessions {
		if session.Kind != "engaged" {
			stats.BackgroundAISeconds += overlapSeconds(session, todayStart, tomorrowStart)
			cursor := dayStart(session.StartAt, location)
			lastDay := dayStart(session.EndAt, location)
			for !cursor.After(lastDay) {
				next := cursor.AddDate(0, 0, 1)
				aiByDate[cursor.Format("2006-01-02")] += overlapSeconds(session, cursor, next)
				cursor = next
			}
			continue
		}
		if !now.Before(session.StartAt) && now.After(session.EndAt) && now.Sub(session.EndAt) <= 90*time.Second {
			live = true
		}
		stats.TotalSeconds += session.Seconds
		today := overlapSeconds(session, todayStart, tomorrowStart)
		stats.TodaySeconds += today
		week := overlapSeconds(session, weekStart, tomorrowStart)
		stats.WeekSeconds += week
		if session.Verification == "review_ready" && week > 0 {
			stats.IterationCount++
		}
		if project := projectTotals[session.ProjectID]; project != nil {
			project.TotalSeconds += session.Seconds
			project.TodaySeconds += today
		}
		cursor := dayStart(session.StartAt, location)
		lastDay := dayStart(session.EndAt, location)
		for !cursor.After(lastDay) {
			next := cursor.AddDate(0, 0, 1)
			seconds := overlapSeconds(session, cursor, next)
			key := cursor.Format("2006-01-02")
			secondsByDate[key] += seconds
			cursor = next
		}
	}
	daily := make([]Day, 0, 7)
	for i := 6; i >= 0; i-- {
		date := todayStart.AddDate(0, 0, -i)
		key := date.Format("2006-01-02")
		daily = append(daily, Day{Date: key, Seconds: secondsByDate[key], AISeconds: aiByDate[key]})
	}
	for cursor := todayStart; ; cursor = cursor.AddDate(0, 0, -1) {
		if secondsByDate[cursor.Format("2006-01-02")] >= 900 {
			stats.StreakDays++
		} else {
			if cursor.Equal(todayStart) {
				continue
			}
			break
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartAt.Equal(sessions[j].StartAt) {
			return sessions[i].ID > sessions[j].ID
		}
		return sessions[i].StartAt.After(sessions[j].StartAt)
	})
	if len(sessions) > 100 {
		sessions = sessions[:100]
	}
	return Dashboard{User: user, Stats: stats, Projects: projects, Daily: daily, Timeline: sessions, Live: live}, nil
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	dashboard, err := s.dashboard(r.Context(), user, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) handlePublicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/api/v1/u/")
	if username == "" || strings.Contains(username, "/") || len(username) > 24 {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	user, err := scanUser(s.db.QueryRowContext(r.Context(), `SELECT `+userColumns+` FROM users WHERE username = ? COLLATE NOCASE AND is_public = 1`, username))
	if err != nil {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	dashboard, err := s.dashboard(r.Context(), user, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load profile")
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}

func (s *Server) publishDashboard(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user, err := s.userByID(ctx, userID)
	if err != nil {
		return
	}
	dashboard, err := s.dashboard(ctx, user, false)
	if err != nil {
		log.Printf("build live dashboard: %v", err)
		return
	}
	payload, err := json.Marshal(dashboard)
	if err == nil {
		s.hub.Publish(userID, payload)
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unavailable")
		return
	}
	messages, cancel := s.hub.Subscribe(user.ID)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	dashboard, err := s.dashboard(r.Context(), user, false)
	if err == nil {
		payload, _ := json.Marshal(dashboard)
		fmt.Fprintf(w, "event: dashboard\ndata: %s\n\n", payload)
	} else {
		fmt.Fprint(w, "event: error\ndata: {\"error\":\"could not load dashboard\"}\n\n")
	}
	flusher.Flush()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload, open := <-messages:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "event: dashboard\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
