package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

func TestLocalAPIRequiresAuthAndRejectsBrowserOrigins(t *testing.T) {
	running, root := newTestAgent(t, "https://example.com")
	body := `{"project_path":` + quoteJSON(root) + `,"source":"vscode","kind":"focus"}`
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:47653/v1/events", strings.NewReader(body))
	response := httptest.NewRecorder()
	NewHandler(running).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || running.queue.Len() != 0 {
		t.Fatalf("missing auth response = %d, queue = %d", response.Code, running.queue.Len())
	}

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:47653/v1/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+running.localToken())
	request.Header.Set("Origin", "https://attacker.example")
	response = httptest.NewRecorder()
	NewHandler(running).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || running.queue.Len() != 0 {
		t.Fatalf("browser origin response = %d, queue = %d", response.Code, running.queue.Len())
	}

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:47653/v1/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+running.localToken())
	response = httptest.NewRecorder()
	NewHandler(running).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || running.queue.Len() != 1 {
		t.Fatalf("valid event response = %d (%s), queue = %d", response.Code, response.Body.String(), running.queue.Len())
	}
	if event := running.queue.First(1)[0]; event.ProjectPath == root || len(event.ProjectPath) != 64 {
		t.Fatalf("raw project path escaped privacy conversion: %#v", event)
	}
}

func TestLocalAPIRejectsUnknownContentFields(t *testing.T) {
	running, root := newTestAgent(t, "https://example.com")
	body := `{"project_path":` + quoteJSON(root) + `,"source":"claude-code","kind":"ai_start","prompt":"never collect this"}`
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:47653/v1/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+running.localToken())
	response := httptest.NewRecorder()
	NewHandler(running).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || running.queue.Len() != 0 {
		t.Fatalf("prompt-bearing event response = %d, queue = %d", response.Code, running.queue.Len())
	}
}

func TestPauseResumeAndStatus(t *testing.T) {
	running, root := newTestAgent(t, "https://example.com")
	fixed := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	running.now = func() time.Time { return fixed }
	if err := running.Pause(time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, _, err := running.Enqueue(tracking.Event{ProjectPath: root, Source: "generic", Kind: "focus"}); err != ErrPaused {
		t.Fatalf("enqueue while paused = %v", err)
	}
	if running.queue.Len() != 0 || !running.Status().Paused {
		t.Fatalf("pause did not stop collection")
	}
	statusRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:47653/v1/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer "+running.localToken())
	statusResponse := httptest.NewRecorder()
	NewHandler(running).ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK || strings.Contains(statusResponse.Body.String(), running.localToken()) || strings.Contains(statusResponse.Body.String(), running.config.DeviceToken) {
		t.Fatalf("unsafe status response: %d %s", statusResponse.Code, statusResponse.Body.String())
	}
	if err := running.Resume(); err != nil {
		t.Fatal(err)
	}
	if _, queued, err := running.Enqueue(tracking.Event{ProjectPath: root, Source: "generic", Kind: "focus"}); err != nil || !queued {
		t.Fatalf("enqueue after resume = %v, %v", queued, err)
	}
}

func TestLoopbackListenerRejectsNonLoopback(t *testing.T) {
	if listener, err := listenLoopback("0.0.0.0:0"); err == nil {
		listener.Close()
		t.Fatal("non-loopback listener was accepted")
	}
	listener, err := listenLoopback("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
}

func newTestAgent(t *testing.T, server string) (*Agent, string) {
	t.Helper()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "agent.json")
	if _, err := Setup(configPath, server, "device-token", nil); err != nil {
		t.Fatal(err)
	}
	running, err := Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Now().UTC()
	running.now = func() time.Time { return fixed }
	return running, t.TempDir()
}

func quoteJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func eventBody(t *testing.T, event tracking.Event) *bytes.Reader {
	t.Helper()
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(encoded)
}
