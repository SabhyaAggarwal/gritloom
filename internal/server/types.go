package server

import "time"

type User struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"display_name"`
	Bio               string    `json:"bio"`
	IsPublic          bool      `json:"is_public"`
	WeeklyGoalSeconds int64     `json:"-"`
	TimeZone          string    `json:"timezone"`
	CreatedAt         time.Time `json:"created_at"`
}

type Project struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Slug          string     `json:"slug"`
	RepositoryURL string     `json:"repository_url"`
	Description   string     `json:"description"`
	Color         string     `json:"color"`
	Language      string     `json:"language"`
	TotalSeconds  int64      `json:"total_seconds"`
	TodaySeconds  int64      `json:"today_seconds"`
	LastActiveAt  *time.Time `json:"last_active_at"`
	IsPublic      bool       `json:"is_public"`
}

type Session struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"project_id"`
	ProjectName     string          `json:"project_name"`
	ProjectColor    string          `json:"project_color"`
	StartAt         time.Time       `json:"start_at"`
	EndAt           time.Time       `json:"end_at"`
	Seconds         int64           `json:"seconds"`
	Kind            string          `json:"kind"`
	Source          string          `json:"source"`
	Verification    string          `json:"verification"`
	EditCount       int             `json:"edit_count"`
	AICount         int             `json:"ai_count"`
	TestCount       int             `json:"test_count"`
	FailedTestCount int             `json:"failed_test_count"`
	CommitCount     int             `json:"commit_count"`
	EvidenceHash    string          `json:"evidence_hash,omitempty"`
	Evidence        []EvidenceEvent `json:"evidence,omitempty"`
}

type EvidenceEvent struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	Source     string    `json:"source"`
	Kind       string    `json:"kind"`
	File       string    `json:"file,omitempty"`
	Additions  int       `json:"additions"`
	Deletions  int       `json:"deletions"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	AI         bool      `json:"ai"`
	Background bool      `json:"background"`
	Model      string    `json:"model,omitempty"`
	CommitHash string    `json:"commit_hash,omitempty"`
	Branch     string    `json:"branch,omitempty"`
}

type Stats struct {
	TodaySeconds        int64 `json:"today_seconds"`
	WeekSeconds         int64 `json:"week_seconds"`
	TotalSeconds        int64 `json:"total_seconds"`
	StreakDays          int   `json:"streak_days"`
	WeeklyGoalSeconds   int64 `json:"weekly_goal_seconds"`
	AISeconds           int64 `json:"ai_seconds"`
	BackgroundAISeconds int64 `json:"background_ai_seconds"`
	IterationCount      int   `json:"iteration_count"`
}

type Day struct {
	Date      string `json:"date"`
	Seconds   int64  `json:"seconds"`
	AISeconds int64  `json:"ai_seconds"`
}

type Dashboard struct {
	User     User      `json:"user"`
	Stats    Stats     `json:"stats"`
	Projects []Project `json:"projects"`
	Daily    []Day     `json:"daily"`
	Timeline []Session `json:"timeline"`
	Live     bool      `json:"live"`
}

type Device struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	LastSeen  *time.Time `json:"last_seen_at,omitempty"`
	Token     string     `json:"token,omitempty"`
}

type Submission struct {
	ID           string    `json:"id"`
	Author       string    `json:"author"`
	Token        string    `json:"token,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Note         string    `json:"note"`
	Seconds      int64     `json:"seconds"`
	SessionCount int       `json:"session_count"`
	Status       string    `json:"status"`
	ManifestHash string    `json:"manifest_hash"`
	Sessions     []Session `json:"sessions"`
}

type Event struct {
	ID            string    `json:"id"`
	OccurredAt    time.Time `json:"occurred_at"`
	ProjectPath   string    `json:"project_path"`
	ProjectName   string    `json:"project_name"`
	RepositoryURL string    `json:"repository_url"`
	Branch        string    `json:"branch"`
	Source        string    `json:"source"`
	Kind          string    `json:"kind"`
	File          string    `json:"file"`
	Additions     int       `json:"additions"`
	Deletions     int       `json:"deletions"`
	ExitCode      *int      `json:"exit_code"`
	AI            bool      `json:"ai"`
	Background    bool      `json:"background"`
	Model         string    `json:"model"`
	CommitHash    string    `json:"commit_hash"`
}

type ingestRequest struct {
	DeviceID string  `json:"device_id"`
	Events   []Event `json:"events"`
}

type storedEvent struct {
	Event
	ProjectID string
}
