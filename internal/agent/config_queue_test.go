package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

func TestSetupPersistsSecretsWith0600AndStableIDs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ACLs do not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "config", "agent.json")
	first, err := Setup(path, "https://example.com/", "device-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
	second, err := Setup(path, "https://example.com", "new-device-token", []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	third, err := Setup(path, "https://example.com", "new-device-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.DeviceID != second.DeviceID || first.LocalToken != second.LocalToken || second.DeviceToken != "new-device-token" {
		t.Fatalf("setup did not preserve stable identifiers: %#v %#v", first, second)
	}
	if len(third.ExcludedProjects) != 1 || !filepath.IsAbs(third.ExcludedProjects[0]) {
		t.Fatalf("setup did not preserve canonical exclusions: %#v", third.ExcludedProjects)
	}
	if len(first.DeviceID) != 32 || len(first.LocalToken) != 43 {
		t.Fatalf("unexpected random ID/token sizes: %#v", first)
	}
}

func TestQueueSurvivesReopenAndAcknowledgesOnlyUploadedPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	queue, err := OpenQueue(path)
	if err != nil {
		t.Fatal(err)
	}
	first := queuedTestEvent("0123456789abcdef", time.Now().UTC())
	second := queuedTestEvent("fedcba9876543210", time.Now().UTC().Add(time.Second))
	if queued, err := queue.Enqueue(first); err != nil || !queued {
		t.Fatalf("enqueue = %v, %v", queued, err)
	}
	if queued, err := queue.Enqueue(first); err != nil || queued {
		t.Fatalf("exact duplicate enqueue = %v, %v", queued, err)
	}
	if queued, err := queue.Enqueue(second); err != nil || !queued {
		t.Fatalf("second enqueue = %v, %v", queued, err)
	}
	reopened, err := OpenQueue(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Len() != 2 || len(reopened.First(1)) != 1 {
		t.Fatalf("queue did not reopen correctly")
	}
	if err := reopened.Acknowledge(1); err != nil {
		t.Fatal(err)
	}
	remaining := reopened.First(10)
	if len(remaining) != 1 || remaining[0].ID != second.ID {
		t.Fatalf("wrong events remain after acknowledgement: %#v", remaining)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("queue mode = %o, want 600", info.Mode().Perm())
		}
	}
}

func queuedTestEvent(id string, occurredAt time.Time) tracking.Event {
	return tracking.Event{
		ID: id, OccurredAt: occurredAt, ProjectPath: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ProjectName: "project", Source: tracking.SourceGeneric, Kind: tracking.KindFocus,
	}
}
