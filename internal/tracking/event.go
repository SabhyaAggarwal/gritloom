// Package tracking provides deterministic, metadata-only activity accounting.
package tracking

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	SourceVSCode     = "vscode"
	SourceClaudeCode = "claude-code"
	SourceCodex      = "codex"
	SourceGeneric    = "generic"

	KindFocus   = "focus"
	KindEdit    = "edit"
	KindSave    = "save"
	KindAIStart = "ai_start"
	KindAIEnd   = "ai_end"
	KindAITool  = "ai_tool"
	KindBuild   = "build"
	KindTest    = "test"
	KindDebug   = "debug"
	KindCommit  = "commit"
)

var (
	eventIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
	projectPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	commitPattern  = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	allowedSources = map[string]bool{SourceVSCode: true, SourceClaudeCode: true, SourceCodex: true, SourceGeneric: true}
	allowedKinds   = map[string]bool{KindFocus: true, KindEdit: true, KindSave: true, KindAIStart: true, KindAIEnd: true, KindAITool: true, KindBuild: true, KindTest: true, KindDebug: true, KindCommit: true}
)

// Event is the privacy-safe event shared with the server. ProjectPath is a
// SHA-256 project identifier, never a filesystem path.
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

// BackgroundAI reports whether an AI lifecycle event belongs to unattended
// work rather than engaged work. Background flags on other event kinds do not
// create AI time.
func (e Event) BackgroundAI() bool {
	if !e.AI || !e.Background {
		return false
	}
	return e.Kind == KindAIStart || e.Kind == KindAIEnd || e.Kind == KindAITool
}

// ValidateEvent checks the server-compatible, privacy-safe event shape. It
// deliberately does not apply a wall-clock timestamp window, so queued events
// can be validated independently of the current time.
func ValidateEvent(e Event) error {
	if !eventIDPattern.MatchString(e.ID) {
		return fmt.Errorf("event id must be 16-64 URL-safe characters")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if !projectPattern.MatchString(e.ProjectPath) {
		return fmt.Errorf("project_path must be exactly a lowercase SHA-256 hash")
	}
	if err := validateText(e.ProjectName, 80, true); err != nil {
		return fmt.Errorf("invalid project_name: %w", err)
	}
	if e.RepositoryURL != "" {
		if len(e.RepositoryURL) > 500 {
			return fmt.Errorf("repository_url is too long")
		}
		u, err := url.Parse(e.RepositoryURL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("repository_url must be a sanitized http(s) URL")
		}
	}
	if err := validateText(e.Branch, 200, false); err != nil {
		return fmt.Errorf("invalid branch: %w", err)
	}
	if !allowedSources[e.Source] {
		return fmt.Errorf("unknown event source")
	}
	if !allowedKinds[e.Kind] {
		return fmt.Errorf("unknown event kind")
	}
	if e.File != "" {
		if len(e.File) > 300 || !utf8.ValidString(e.File) || strings.Contains(e.File, "\\") || strings.HasPrefix(e.File, "/") || path.Clean(e.File) != e.File || e.File == "." || e.File == ".." || strings.HasPrefix(e.File, "../") || strings.IndexFunc(e.File, unicode.IsControl) >= 0 {
			return fmt.Errorf("file must be a clean repository-relative path")
		}
	}
	if e.Additions < 0 || e.Deletions < 0 || e.Additions > 10_000_000 || e.Deletions > 10_000_000 {
		return fmt.Errorf("invalid change counts")
	}
	if err := validateText(e.Model, 100, false); err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}
	if e.CommitHash != "" && !commitPattern.MatchString(e.CommitHash) {
		return fmt.Errorf("invalid commit_hash")
	}
	return nil
}

func validateText(value string, maximum int, required bool) error {
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is required")
	}
	if len(value) > maximum || !utf8.ValidString(value) {
		return fmt.Errorf("value exceeds %d bytes or is not UTF-8", maximum)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("value contains control characters")
	}
	return nil
}
