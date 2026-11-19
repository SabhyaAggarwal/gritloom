package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

func TestBatchedDeviceUploadAndOfflineRetention(t *testing.T) {
	var mu sync.Mutex
	var requests []ingestRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/ingest" || request.Header.Get("Authorization") != "Bearer device-token" {
			t.Errorf("bad ingest path or auth: %s %s", request.URL.Path, request.Header.Get("Authorization"))
		}
		var input ingestRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Errorf("decode ingest: %v", err)
		}
		mu.Lock()
		requests = append(requests, input)
		responseCount := len(requests)
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if responseCount == 1 {
			_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": 2, "duplicates": 0})
		} else {
			_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": 0, "duplicates": 2})
		}
	}))
	defer server.Close()

	running, root := newTestAgent(t, server.URL)
	for _, id := range []string{"0123456789abcdef", "fedcba9876543210"} {
		_, queued, err := running.Enqueue(tracking.Event{ID: id, ProjectPath: root, Source: "generic", Kind: "focus"})
		if err != nil || !queued {
			t.Fatalf("enqueue %s = %v, %v", id, queued, err)
		}
	}
	// Keep the same queue while the configured server is unavailable.
	running.config.Server = "http://127.0.0.1:1"
	if err := running.Flush(context.Background()); err == nil || running.queue.Len() != 2 {
		t.Fatalf("offline flush error = %v, queue = %d", err, running.queue.Len())
	}
	running.config.Server = server.URL
	if err := running.Flush(context.Background()); err != nil || running.queue.Len() != 0 {
		t.Fatalf("online flush error = %v, queue = %d", err, running.queue.Len())
	}

	mu.Lock()
	if len(requests) != 1 || requests[0].DeviceID != running.config.DeviceID || len(requests[0].Events) != 2 {
		t.Fatalf("unexpected ingest requests: %#v", requests)
	}
	encoded, _ := json.Marshal(requests[0])
	if strings.Contains(string(encoded), root) {
		t.Fatalf("upload contains raw project path: %s", encoded)
	}
	mu.Unlock()

	// Reopening a crash-era queue and receiving duplicate acknowledgements also
	// removes the confirmed events.
	if _, err := running.queue.Enqueue(queuedTestEvent("0123456789abcdef", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if _, err := running.queue.Enqueue(queuedTestEvent("fedcba9876543210", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if err := running.Flush(context.Background()); err != nil || running.queue.Len() != 0 {
		t.Fatalf("duplicate flush error = %v, queue = %d", err, running.queue.Len())
	}
	status := running.Status()
	if status.Accepted != 2 || status.Duplicates != 2 || status.LastUploadError != "" {
		t.Fatalf("unexpected upload status: %#v", status)
	}
}
