package tracking

import (
	"encoding/json"
	"sort"
	"time"
)

const (
	// EngagedMaxGap is the largest idle gap bridged between two foreground
	// signals. Longer gaps start a new session instead of inventing activity.
	// Background AI has a longer bound for ordinary unattended tool runs.
	EngagedMaxGap    = 2 * time.Minute
	BackgroundMaxGap = 15 * time.Minute

	SessionEngaged      = "engaged"
	SessionBackgroundAI = "background_ai"
)

// EvidenceEvent retains useful proof of activity without prompt or source
// contents. All fields have already passed the agent's privacy boundary.
type EvidenceEvent struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	Source     string    `json:"source"`
	Kind       string    `json:"kind"`
	File       string    `json:"file"`
	Additions  int       `json:"additions"`
	Deletions  int       `json:"deletions"`
	ExitCode   *int      `json:"exit_code"`
	AI         bool      `json:"ai"`
	Background bool      `json:"background"`
	Model      string    `json:"model"`
	CommitHash string    `json:"commit_hash"`
	Branch     string    `json:"branch"`
}

// Session is a merged, non-overlapping interval for one project and one kind
// of work. Sources and Evidence provide an auditable summary of the interval.
type Session struct {
	ProjectPath string          `json:"project_path"`
	StartAt     time.Time       `json:"start_at"`
	EndAt       time.Time       `json:"end_at"`
	Duration    time.Duration   `json:"-"`
	Seconds     int64           `json:"seconds"`
	Kind        string          `json:"kind"`
	Sources     []string        `json:"sources"`
	Evidence    []EvidenceEvent `json:"evidence"`
}

// ProjectSummary keeps foreground and unattended AI accounting separate.
type ProjectSummary struct {
	EngagedDuration      time.Duration `json:"-"`
	BackgroundAIDuration time.Duration `json:"-"`
	EngagedSeconds       int64         `json:"engaged_seconds"`
	BackgroundAISeconds  int64         `json:"background_ai_seconds"`
}

// Summary is a deterministic accounting report for a collection of events.
type Summary struct {
	EngagedDuration      time.Duration             `json:"-"`
	BackgroundAIDuration time.Duration             `json:"-"`
	EngagedSeconds       int64                     `json:"engaged_seconds"`
	BackgroundAISeconds  int64                     `json:"background_ai_seconds"`
	ByProject            map[string]ProjectSummary `json:"by_project"`
	Sessions             []Session                 `json:"sessions"`
}

type segment struct {
	project    string
	kind       string
	start, end time.Time
}

// BuildSessions sorts a copy of events, making input order irrelevant. A
// single foreground timeline unions all editor and harness sources, so their
// overlapping signals are never counted twice. Background AI is accounted
// separately per project because independent unattended jobs can run in
// parallel.
func BuildSessions(events []Event) []Session {
	ordered := orderedEvents(events)
	foreground := make([]Event, 0, len(ordered))
	background := make(map[string][]Event)
	for _, event := range ordered {
		if event.BackgroundAI() {
			background[event.ProjectPath] = append(background[event.ProjectPath], event)
		} else {
			foreground = append(foreground, event)
		}
	}

	segments := make([]segment, 0, len(ordered))
	for i := 0; i+1 < len(foreground); i++ {
		end := foreground[i+1].OccurredAt
		// A project switch does not prove that the preceding gap belonged to
		// either project. Count only intervals bounded by the same project.
		if foreground[i].ProjectPath != foreground[i+1].ProjectPath {
			continue
		}
		if end.Sub(foreground[i].OccurredAt) <= EngagedMaxGap {
			segments = appendMerged(segments, segment{project: foreground[i].ProjectPath, kind: SessionEngaged, start: foreground[i].OccurredAt, end: end})
		}
	}
	for _, projectEvents := range background {
		projectSegments := make([]segment, 0, len(projectEvents))
		for i := 0; i+1 < len(projectEvents); i++ {
			end := projectEvents[i+1].OccurredAt
			if end.Sub(projectEvents[i].OccurredAt) <= BackgroundMaxGap {
				projectSegments = appendMerged(projectSegments, segment{project: projectEvents[i].ProjectPath, kind: SessionBackgroundAI, start: projectEvents[i].OccurredAt, end: end})
			}
		}
		segments = append(segments, projectSegments...)
	}

	sort.Slice(segments, func(i, j int) bool {
		if !segments[i].start.Equal(segments[j].start) {
			return segments[i].start.Before(segments[j].start)
		}
		if !segments[i].end.Equal(segments[j].end) {
			return segments[i].end.Before(segments[j].end)
		}
		if segments[i].project != segments[j].project {
			return segments[i].project < segments[j].project
		}
		return segments[i].kind < segments[j].kind
	})

	type groupKey struct {
		project    string
		background bool
	}
	groups := make(map[groupKey][]Event)
	for _, event := range ordered {
		key := groupKey{project: event.ProjectPath, background: event.BackgroundAI()}
		groups[key] = append(groups[key], event)
	}

	sessions := make([]Session, 0, len(segments))
	for _, part := range segments {
		if !part.start.Before(part.end) {
			continue
		}
		session := Session{
			ProjectPath: part.project,
			StartAt:     part.start,
			EndAt:       part.end,
			Duration:    part.end.Sub(part.start),
			Seconds:     int64(part.end.Sub(part.start) / time.Second),
			Kind:        part.kind,
			Sources:     []string{},
			Evidence:    []EvidenceEvent{},
		}
		key := groupKey{project: part.project, background: part.kind == SessionBackgroundAI}
		group := groups[key]
		start := sort.Search(len(group), func(i int) bool { return !group[i].OccurredAt.Before(part.start) })
		sourceSet := make(map[string]struct{})
		for _, event := range group[start:] {
			if event.OccurredAt.After(part.end) {
				break
			}
			sourceSet[event.Source] = struct{}{}
			session.Evidence = append(session.Evidence, evidenceFor(event))
		}
		for source := range sourceSet {
			session.Sources = append(session.Sources, source)
		}
		sort.Strings(session.Sources)
		sessions = append(sessions, session)
	}
	return sessions
}

// Summarize accounts events and aggregates exact durations. Seconds fields are
// rounded down only after durations have been summed, avoiding per-session
// rounding drift.
func Summarize(events []Event) Summary {
	sessions := BuildSessions(events)
	summary := Summary{ByProject: make(map[string]ProjectSummary), Sessions: sessions}
	for _, session := range sessions {
		project := summary.ByProject[session.ProjectPath]
		if session.Kind == SessionBackgroundAI {
			summary.BackgroundAIDuration += session.Duration
			project.BackgroundAIDuration += session.Duration
		} else {
			summary.EngagedDuration += session.Duration
			project.EngagedDuration += session.Duration
		}
		summary.ByProject[session.ProjectPath] = project
	}
	summary.EngagedSeconds = int64(summary.EngagedDuration / time.Second)
	summary.BackgroundAISeconds = int64(summary.BackgroundAIDuration / time.Second)
	for key, project := range summary.ByProject {
		project.EngagedSeconds = int64(project.EngagedDuration / time.Second)
		project.BackgroundAISeconds = int64(project.BackgroundAIDuration / time.Second)
		summary.ByProject[key] = project
	}
	return summary
}

func orderedEvents(events []Event) []Event {
	type sortableEvent struct {
		event     Event
		canonical string
	}
	ordered := make([]sortableEvent, 0, len(events))
	for _, event := range events {
		if event.OccurredAt.IsZero() {
			continue
		}
		event.OccurredAt = event.OccurredAt.UTC().Round(0)
		encoded, err := json.Marshal(event)
		if err != nil {
			continue
		}
		ordered = append(ordered, sortableEvent{event: event, canonical: string(encoded)})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].event.OccurredAt.Equal(ordered[j].event.OccurredAt) {
			return ordered[i].event.OccurredAt.Before(ordered[j].event.OccurredAt)
		}
		if ordered[i].event.ID != ordered[j].event.ID {
			return ordered[i].event.ID < ordered[j].event.ID
		}
		return ordered[i].canonical < ordered[j].canonical
	})
	result := make([]Event, 0, len(ordered))
	for i, item := range ordered {
		// Replaying an identical local event must not duplicate evidence.
		if i > 0 && item.event.ID != "" && ordered[i-1].event.ID == item.event.ID && ordered[i-1].canonical == item.canonical {
			continue
		}
		result = append(result, item.event)
	}
	return result
}

func appendMerged(segments []segment, next segment) []segment {
	if !next.start.Before(next.end) {
		return segments
	}
	if len(segments) > 0 {
		last := &segments[len(segments)-1]
		if last.project == next.project && last.kind == next.kind && !next.start.After(last.end) {
			if next.end.After(last.end) {
				last.end = next.end
			}
			return segments
		}
	}
	return append(segments, next)
}

func evidenceFor(event Event) EvidenceEvent {
	evidence := EvidenceEvent{
		ID: event.ID, OccurredAt: event.OccurredAt, Source: event.Source, Kind: event.Kind,
		File: event.File, Additions: event.Additions, Deletions: event.Deletions,
		AI: event.AI, Background: event.Background, Model: event.Model,
		CommitHash: event.CommitHash, Branch: event.Branch,
	}
	if event.ExitCode != nil {
		value := *event.ExitCode
		evidence.ExitCode = &value
	}
	return evidence
}
