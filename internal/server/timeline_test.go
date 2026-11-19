package server

import (
	"testing"
	"time"
)

func timelineEvent(id, project string, offset int, kind string) storedEvent {
	return storedEvent{Event: Event{ID: id, OccurredAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Add(time.Duration(offset) * time.Second), Source: "vscode", Kind: kind}, ProjectID: project}
}

func totalSessionSeconds(sessions []Session, kind string) int64 {
	var total int64
	for _, session := range sessions {
		if kind == "" || session.Kind == kind {
			total += session.Seconds
		}
	}
	return total
}

func TestBuildSessionsCountsEngagedTimeConservatively(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "focus"),
		timelineEvent("0000000000000002", "project-a", 30, "edit"),
		timelineEvent("0000000000000003", "project-a", 60, "focus"),
		timelineEvent("0000000000000004", "project-a", 600, "focus"),
	}
	sessions := buildSessions("user", events)
	if got := totalSessionSeconds(sessions, "engaged"); got != 60 {
		t.Fatalf("engaged seconds = %d, want 60; sessions: %#v", got, sessions)
	}
}

func TestBuildSessionsMergesOverlappingSources(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "focus"),
		timelineEvent("0000000000000002", "project-a", 30, "ai_start"),
		timelineEvent("0000000000000003", "project-a", 60, "edit"),
		timelineEvent("0000000000000004", "project-a", 90, "focus"),
	}
	events[1].Source = "claude-code"
	sessions := buildSessions("user", events)
	if got := totalSessionSeconds(sessions, "engaged"); got != 90 {
		t.Fatalf("overlapping sources counted %d seconds, want 90", got)
	}
}

func TestBuildSessionsAttributesProjectSwitches(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "focus"),
		timelineEvent("0000000000000002", "project-b", 60, "focus"),
		timelineEvent("0000000000000003", "project-b", 120, "edit"),
	}
	sessions := buildSessions("user", events)
	if len(sessions) != 1 || sessions[0].ProjectID != "project-b" || sessions[0].Seconds != 60 {
		t.Fatalf("unexpected project attribution: %#v", sessions)
	}
}

func TestBuildSessionsSeparatesBackgroundAI(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "focus"),
		timelineEvent("0000000000000002", "project-a", 30, "ai_start"),
		timelineEvent("0000000000000003", "project-a", 60, "ai_tool"),
		timelineEvent("0000000000000004", "project-a", 120, "ai_end"),
	}
	for i := 1; i < len(events); i++ {
		events[i].Background = true
		events[i].AI = true
	}
	sessions := buildSessions("user", events)
	if got := totalSessionSeconds(sessions, "engaged"); got != 0 {
		t.Fatalf("single focus event inferred %d engaged seconds from background AI", got)
	}
	if got := totalSessionSeconds(sessions, "background_ai"); got != 90 {
		t.Fatalf("background = %d, want 90", got)
	}

	events = append(events, timelineEvent("0000000000000005", "project-a", 120, "focus"))
	sessions = buildSessions("user", events)
	if got := totalSessionSeconds(sessions, "engaged"); got != 120 {
		t.Fatalf("background AI interrupted foreground session: engaged = %d, want 120", got)
	}
	if got := totalSessionSeconds(sessions, "background_ai"); got != 90 {
		t.Fatalf("background = %d after foreground event, want 90", got)
	}
}

func TestBuildSessionsSortsOutOfOrderEvents(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000003", "project-a", 60, "focus"),
		timelineEvent("0000000000000001", "project-a", 0, "focus"),
		timelineEvent("0000000000000002", "project-a", 30, "edit"),
	}
	sessions := buildSessions("user", events)
	if got := totalSessionSeconds(sessions, "engaged"); got != 60 {
		t.Fatalf("out-of-order events counted %d seconds, want 60", got)
	}
}

func TestBuildSessionsRecognizesIterationEvidence(t *testing.T) {
	failure, success := 1, 0
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "edit"),
		timelineEvent("0000000000000002", "project-a", 30, "test"),
		timelineEvent("0000000000000003", "project-a", 60, "test"),
		timelineEvent("0000000000000004", "project-a", 90, "commit"),
	}
	events[1].ExitCode = &failure
	events[2].ExitCode = &success
	events[3].CommitHash = "abcdef1234567"
	sessions := buildSessions("user", events)
	if len(sessions) != 1 || sessions[0].Verification != "review_ready" || sessions[0].FailedTestCount != 1 || sessions[0].CommitCount != 1 {
		t.Fatalf("iteration evidence not recognized: %#v", sessions)
	}
	if len(sessions[0].Evidence) != 4 || sessions[0].EvidenceHash == "" {
		t.Fatalf("evidence details missing: %#v", sessions[0])
	}
}

func TestBuildSessionsRejectsLongBackgroundGap(t *testing.T) {
	events := []storedEvent{
		timelineEvent("0000000000000001", "project-a", 0, "ai_start"),
		timelineEvent("0000000000000002", "project-a", 1200, "ai_end"),
	}
	for i := range events {
		events[i].Background = true
	}
	if got := totalSessionSeconds(buildSessions("user", events), "background_ai"); got != 0 {
		t.Fatalf("long unattended gap counted %d seconds", got)
	}
}
