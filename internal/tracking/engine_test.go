package tracking

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

var trackingStart = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func trackingEvent(id, project, source string, at time.Time) Event {
	return Event{ID: id, ProjectPath: project, ProjectName: "project", OccurredAt: at, Source: source, Kind: KindEdit}
}

func TestEngagedIdleGapIsCapped(t *testing.T) {
	events := []Event{
		trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart),
		trackingEvent("0000000000000002", "a", SourceVSCode, trackingStart.Add(time.Minute)),
		trackingEvent("0000000000000003", "a", SourceVSCode, trackingStart.Add(10*time.Minute)),
		trackingEvent("0000000000000004", "a", SourceVSCode, trackingStart.Add(10*time.Minute+30*time.Second)),
	}
	summary := Summarize(events)
	if got, want := summary.EngagedDuration, time.Minute+30*time.Second; got != want {
		t.Fatalf("engaged duration = %v, want %v (long idle gaps must not be credited)", got, want)
	}
	if got, want := len(summary.Sessions), 2; got != want {
		t.Fatalf("sessions = %d, want %d: %#v", got, want, summary.Sessions)
	}
}

func TestSwitchingProjectsDoesNotAttributeUnknownGap(t *testing.T) {
	events := []Event{
		trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart),
		trackingEvent("0000000000000002", "b", SourceVSCode, trackingStart.Add(time.Minute)),
		trackingEvent("0000000000000003", "b", SourceVSCode, trackingStart.Add(2*time.Minute)),
	}
	sessions := BuildSessions(events)
	if len(sessions) != 1 || sessions[0].ProjectPath != "b" || sessions[0].Duration != time.Minute {
		t.Fatalf("incorrect project intervals: %#v", sessions)
	}
}

func TestOverlappingSourcesAreUnioned(t *testing.T) {
	events := []Event{
		trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart),
		trackingEvent("0000000000000002", "a", SourceClaudeCode, trackingStart.Add(30*time.Second)),
		trackingEvent("0000000000000003", "a", SourceCodex, trackingStart.Add(time.Minute)),
		trackingEvent("0000000000000004", "a", SourceVSCode, trackingStart.Add(90*time.Second)),
	}
	summary := Summarize(events)
	if got := summary.EngagedDuration; got != 90*time.Second {
		t.Fatalf("engaged duration = %v, want 1m30s", got)
	}
	if len(summary.Sessions) != 1 || !reflect.DeepEqual(summary.Sessions[0].Sources, []string{SourceClaudeCode, SourceCodex, SourceVSCode}) {
		t.Fatalf("sources were not retained deterministically: %#v", summary.Sessions)
	}
}

func TestBackgroundAIIsSeparateAndDoesNotInterruptForeground(t *testing.T) {
	background := func(id string, at time.Time) Event {
		event := trackingEvent(id, "a", SourceClaudeCode, at)
		event.Kind = KindAITool
		event.AI = true
		event.Background = true
		return event
	}
	events := []Event{
		trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart),
		trackingEvent("0000000000000002", "a", SourceVSCode, trackingStart.Add(2*time.Minute)),
		background("0000000000000003", trackingStart.Add(30*time.Second)),
		background("0000000000000004", trackingStart.Add(90*time.Second)),
	}
	summary := Summarize(events)
	if summary.EngagedDuration != 2*time.Minute || summary.BackgroundAIDuration != time.Minute {
		t.Fatalf("incorrect split: engaged=%v background=%v", summary.EngagedDuration, summary.BackgroundAIDuration)
	}
	if len(summary.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(summary.Sessions))
	}
}

func TestBackgroundAIInSeparateProjectsCanOverlap(t *testing.T) {
	events := []Event{}
	for i, project := range []string{"a", "b"} {
		start := trackingEvent(fmt.Sprintf("000000000000000%d", i*2+1), project, SourceCodex, trackingStart)
		start.Kind, start.AI, start.Background = KindAIStart, true, true
		end := trackingEvent(fmt.Sprintf("000000000000000%d", i*2+2), project, SourceCodex, trackingStart.Add(5*time.Minute))
		end.Kind, end.AI, end.Background = KindAIEnd, true, true
		events = append(events, start, end)
	}
	summary := Summarize(events)
	if summary.BackgroundAIDuration != 10*time.Minute || summary.EngagedDuration != 0 {
		t.Fatalf("background duration = %v, want 10m", summary.BackgroundAIDuration)
	}
}

func TestOutOfOrderInputIsDeterministic(t *testing.T) {
	events := []Event{
		trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart),
		trackingEvent("0000000000000002", "b", SourceClaudeCode, trackingStart.Add(20*time.Second)),
		trackingEvent("0000000000000003", "a", SourceCodex, trackingStart.Add(40*time.Second)),
		trackingEvent("0000000000000004", "a", SourceVSCode, trackingStart.Add(5*time.Minute)),
		trackingEvent("0000000000000005", "a", SourceVSCode, trackingStart.Add(5*time.Minute+20*time.Second)),
	}
	want := Summarize(events)
	shuffled := []Event{events[3], events[1], events[4], events[0], events[2]}
	got := Summarize(shuffled)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("out-of-order input changed accounting\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestBackgroundFlagOnHumanEventIsNotAI(t *testing.T) {
	for _, ai := range []bool{false, true} {
		first := trackingEvent("0000000000000001", "a", SourceVSCode, trackingStart)
		first.Background, first.AI = true, ai
		second := trackingEvent("0000000000000002", "a", SourceVSCode, trackingStart.Add(time.Minute))
		second.Background, second.AI = true, ai
		summary := Summarize([]Event{first, second})
		if summary.EngagedDuration != time.Minute || summary.BackgroundAIDuration != 0 {
			t.Fatalf("human event was misclassified with ai=%v: %#v", ai, summary)
		}
	}
}
