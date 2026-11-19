package agent

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

var (
	ErrExcluded    = errors.New("project path is excluded")
	eventIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
)

// Resolver performs the only raw-path to privacy-safe event conversion. Raw
// paths are used locally for exclusion and git discovery, then discarded.
type Resolver struct {
	excluded []string
}

func NewResolver(excluded []string) (*Resolver, error) {
	resolver := &Resolver{}
	for _, excludedPath := range excluded {
		canonical, err := canonicalLocalPath(excludedPath)
		if err != nil {
			return nil, fmt.Errorf("invalid excluded project path: %w", err)
		}
		resolver.excluded = append(resolver.excluded, canonical)
	}
	return resolver, nil
}

// ResolveEvent applies the server's timestamp and ID rules, infers project
// metadata, and returns an event containing no absolute local path.
func (r *Resolver) ResolveEvent(raw tracking.Event, now time.Time) (tracking.Event, error) {
	var event tracking.Event
	if strings.TrimSpace(raw.ProjectPath) == "" {
		return event, fmt.Errorf("project_path is required")
	}
	inputPath, err := canonicalLocalPath(raw.ProjectPath)
	if err != nil {
		return event, err
	}
	if r.excludes(inputPath) {
		return event, ErrExcluded
	}
	project, err := resolveProject(inputPath)
	if err != nil {
		return event, err
	}
	if r.excludes(project.Root) {
		return event, ErrExcluded
	}

	now = now.UTC()
	event = raw
	event.OccurredAt = event.OccurredAt.UTC().Round(0)
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	if event.OccurredAt.After(now.Add(10*time.Minute)) || event.OccurredAt.Before(now.AddDate(-2, 0, 0)) {
		return tracking.Event{}, fmt.Errorf("event timestamp is outside the accepted range")
	}
	if event.ID == "" {
		event.ID, err = randomHex(16)
		if err != nil {
			return tracking.Event{}, err
		}
	} else if !eventIDPattern.MatchString(event.ID) {
		return tracking.Event{}, fmt.Errorf("event id must be 16-64 URL-safe characters")
	}
	if event.Source == "" {
		event.Source = tracking.SourceGeneric
	}
	if event.Kind == tracking.KindAIStart || event.Kind == tracking.KindAIEnd || event.Kind == tracking.KindAITool {
		event.AI = true
	}
	event.ProjectPath = project.Hash
	event.ProjectName = project.Name
	event.RepositoryURL = project.RepositoryURL
	event.Branch = project.Branch
	event.File, err = repositoryRelativeFile(raw.File, project.Root)
	if err != nil {
		return tracking.Event{}, err
	}
	event.Model = strings.TrimSpace(event.Model)
	if err := tracking.ValidateEvent(event); err != nil {
		return tracking.Event{}, err
	}
	return event, nil
}

func canonicalLocalPath(value string) (string, error) {
	if value == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("invalid local project path")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve local project path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	return absolute, nil
}

func (r *Resolver) excludes(candidate string) bool {
	for _, excluded := range r.excluded {
		if pathWithin(excluded, candidate) {
			return true
		}
	}
	return false
}

func pathWithin(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func repositoryRelativeFile(value, root string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("invalid file metadata")
	}
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		absolute, err := canonicalLocalPath(value)
		if err != nil {
			return "", err
		}
		if !pathWithin(root, absolute) {
			return "", fmt.Errorf("absolute file path is outside the project")
		}
		relative, err := filepath.Rel(root, absolute)
		if err != nil {
			return "", fmt.Errorf("resolve repository-relative file: %w", err)
		}
		value = relative
	}
	value = filepath.ToSlash(value)
	value = strings.ReplaceAll(value, "\\", "/")
	windowsDrivePath := len(value) >= 3 && value[1] == ':' && value[2] == '/' && (value[0] >= 'A' && value[0] <= 'Z' || value[0] >= 'a' && value[0] <= 'z')
	if len(value) > 300 || strings.HasPrefix(value, "/") || windowsDrivePath {
		return "", fmt.Errorf("file must be a repository-relative path")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value {
		return "", fmt.Errorf("file must be a clean repository-relative path")
	}
	return clean, nil
}
