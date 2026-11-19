package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/database"
)

type testAPI struct {
	baseURL string
	server  *Server
	client  *http.Client
}

type testResponse struct {
	status  int
	body    []byte
	header  http.Header
	cookies []*http.Cookie
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir()+"/gritloom.db")
	if err != nil {
		t.Fatal(err)
	}
	application, err := New(db, Config{Development: true})
	if err != nil {
		t.Fatal(err)
	}
	application.now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }
	httpServer := httptest.NewServer(application.Handler())
	t.Cleanup(func() { httpServer.Close(); db.Close() })
	jar, _ := cookiejar.New(nil)
	return &testAPI{baseURL: httpServer.URL, server: application, client: &http.Client{Jar: jar, Timeout: 5 * time.Second}}
}

func (api *testAPI) request(t *testing.T, client *http.Client, method, path string, input any, token, origin string) testResponse {
	t.Helper()
	var body bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = *bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, api.baseURL+path, &body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	return testResponse{status: response.StatusCode, body: buffer.Bytes(), header: response.Header, cookies: response.Cookies()}
}

func decodeResponse[T any](t *testing.T, response testResponse) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response.body, &value); err != nil {
		t.Fatalf("decode response %d %s: %v", response.status, response.body, err)
	}
	return value
}

func requireStatus(t *testing.T, response testResponse, expected int) {
	t.Helper()
	if response.status != expected {
		t.Fatalf("status = %d, want %d: %s", response.status, expected, response.body)
	}
}

func TestAccountIngestSubmissionPrivacyJourney(t *testing.T) {
	api := newTestAPI(t)
	register := map[string]string{"username": "builder", "display_name": "Builder", "password": "correct-horse-123"}
	response := api.request(t, api.client, http.MethodPost, "/api/v1/auth/register", register, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	if len(response.cookies) != 1 || !response.cookies[0].HttpOnly || response.cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe session cookie: %#v", response.cookies)
	}
	if response.header.Get("X-Content-Type-Options") != "nosniff" || response.header.Get("Content-Security-Policy") == "" {
		t.Fatal("security headers missing")
	}

	response = api.request(t, api.client, http.MethodPatch, "/api/v1/me", map[string]any{"timezone": "Asia/Kolkata", "weekly_goal_seconds": 36000}, "", api.baseURL)
	requireStatus(t, response, http.StatusOK)
	if response.status != http.StatusOK {
		t.Fatalf("timezone update failed: %s", response.body)
	}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/devices", map[string]string{"name": "Linux laptop"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	device := decodeResponse[Device](t, response)
	if device.Token == "" || len(device.Token) != 43 {
		t.Fatalf("device token missing: %#v", device)
	}

	start := api.server.now().Add(-10 * time.Minute)
	failure, success := 1, 0
	events := []Event{
		{ID: "event-0000000001", OccurredAt: start, ProjectPath: strings.Repeat("a", 64), ProjectName: "Orbit Game", RepositoryURL: "https://github.com/builder/orbit", Source: "vscode", Kind: "edit", File: "src/auth.ts", Additions: 12},
		{ID: "event-0000000002", OccurredAt: start.Add(30 * time.Second), ProjectPath: strings.Repeat("a", 64), ProjectName: "Orbit Game", RepositoryURL: "https://github.com/builder/orbit", Source: "vscode", Kind: "test", File: "src/auth.ts", ExitCode: &failure},
		{ID: "event-0000000003", OccurredAt: start.Add(60 * time.Second), ProjectPath: strings.Repeat("a", 64), ProjectName: "Orbit Game", RepositoryURL: "https://github.com/builder/orbit", Source: "vscode", Kind: "test", File: "src/auth.ts", ExitCode: &success},
		{ID: "event-0000000004", OccurredAt: start.Add(90 * time.Second), ProjectPath: strings.Repeat("a", 64), ProjectName: "Orbit Game", RepositoryURL: "https://github.com/builder/orbit", Source: "generic", Kind: "commit", CommitHash: "abcdef1234567"},
	}
	ingest := ingestRequest{DeviceID: "stable-device", Events: events}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingest, device.Token, "")
	requireStatus(t, response, http.StatusOK)
	result := decodeResponse[map[string]int](t, response)
	if result["accepted"] != 4 || result["duplicates"] != 0 {
		t.Fatalf("unexpected ingest result: %#v", result)
	}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingest, device.Token, "")
	requireStatus(t, response, http.StatusOK)
	result = decodeResponse[map[string]int](t, response)
	if result["accepted"] != 0 || result["duplicates"] != 4 {
		t.Fatalf("retry was not idempotent: %#v", result)
	}
	conflict := ingest
	conflict.Events = append([]Event(nil), events...)
	conflict.Events[0].Additions++
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", conflict, device.Token, "")
	requireStatus(t, response, http.StatusConflict)

	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	dashboard := decodeResponse[Dashboard](t, response)
	if dashboard.Stats.TotalSeconds != 90 || dashboard.Stats.TodaySeconds != 90 || dashboard.Stats.IterationCount != 1 || len(dashboard.Timeline) != 1 || dashboard.Timeline[0].Verification != "review_ready" {
		t.Fatalf("unexpected dashboard: %#v", dashboard)
	}
	if len(dashboard.Timeline[0].Evidence) != 0 {
		t.Fatal("ordinary dashboard leaked detailed evidence")
	}
	sessionID := dashboard.Timeline[0].ID

	response = api.request(t, api.client, http.MethodPost, "/api/v1/submissions", map[string]any{"session_ids": []string{sessionID}, "note": "Fixed session expiry after two failing attempts."}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	submission := decodeResponse[Submission](t, response)
	if submission.Seconds != 90 || len(submission.Sessions) != 1 || len(submission.Sessions[0].Evidence) != 4 || submission.Token == "" {
		t.Fatalf("incomplete submission: %#v", submission)
	}
	if hash, err := verifyManifest(submission); err != nil || hash != submission.ManifestHash {
		t.Fatalf("created manifest does not verify: %s %s %v", hash, submission.ManifestHash, err)
	}
	response = api.request(t, api.client, http.MethodGet, "/api/v1/submissions/"+submission.Token, nil, "", "")
	requireStatus(t, response, http.StatusOK)
	review := decodeResponse[Submission](t, response)
	if hash, err := verifyManifest(review); err != nil || hash != review.ManifestHash {
		t.Fatalf("reloaded manifest does not verify: %s %s %v", hash, review.ManifestHash, err)
	}
	response = api.request(t, api.client, http.MethodGet, "/api/v1/submissions/"+submission.Token+"/manifest", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	digest := sha256.Sum256(response.body)
	if hash := hex.EncodeToString(digest[:]); hash != submission.ManifestHash || response.header.Get("X-Gritloom-Manifest-SHA256") != hash {
		t.Fatalf("raw manifest hash mismatch: %s vs %s", hash, submission.ManifestHash)
	}
	var rawManifest struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(response.body, &rawManifest); err != nil || rawManifest.Version != 1 {
		t.Fatalf("invalid raw manifest: %s %v", response.body, err)
	}

	extension := Event{ID: "event-0000000005", OccurredAt: start.Add(120 * time.Second), ProjectPath: strings.Repeat("a", 64), ProjectName: "Orbit Game", Source: "vscode", Kind: "focus"}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable-device", Events: []Event{extension}}, device.Token, "")
	requireStatus(t, response, http.StatusOK)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/submissions/"+submission.Token, nil, "", "")
	review = decodeResponse[Submission](t, response)
	if review.Seconds != 90 || review.Sessions[0].EndAt.Sub(review.Sessions[0].StartAt) != 90*time.Second {
		t.Fatalf("submission changed after new activity: %#v", review)
	}

	response = api.request(t, api.client, http.MethodPatch, "/api/v1/me", map[string]bool{"is_public": false}, "", "https://attacker.invalid")
	requireStatus(t, response, http.StatusForbidden)
	response = api.request(t, api.client, http.MethodPatch, "/api/v1/me", map[string]bool{"is_public": false}, "", api.baseURL)
	requireStatus(t, response, http.StatusOK)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/u/builder", nil, "", "")
	requireStatus(t, response, http.StatusNotFound)

	response = api.request(t, api.client, http.MethodDelete, "/api/v1/devices/"+device.ID, nil, "", api.baseURL)
	requireStatus(t, response, http.StatusNoContent)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable-device", Events: []Event{extension}}, device.Token, "")
	requireStatus(t, response, http.StatusUnauthorized)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	dashboard = decodeResponse[Dashboard](t, response)
	if dashboard.Stats.TotalSeconds != 120 {
		t.Fatalf("device revocation removed history: %#v", dashboard.Stats)
	}
}

func TestIngestRejectsRawPathsAndInvalidDevice(t *testing.T) {
	api := newTestAPI(t)
	response := api.request(t, api.client, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "privacy", "display_name": "Privacy", "password": "private-pass-123"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/devices", map[string]string{"name": "Laptop"}, "", api.baseURL)
	device := decodeResponse[Device](t, response)
	event := Event{ID: "event-0000000001", OccurredAt: api.server.now(), ProjectPath: "/home/user/private-project", ProjectName: "Private", Source: "vscode", Kind: "focus"}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable", Events: []Event{event}}, device.Token, "")
	requireStatus(t, response, http.StatusBadRequest)
	event.ProjectPath = strings.Repeat("b", 64)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable", Events: []Event{event}}, "invalid-token", "")
	requireStatus(t, response, http.StatusUnauthorized)
}

func TestPublicProfileOmitsPrivateProjects(t *testing.T) {
	api := newTestAPI(t)
	response := api.request(t, api.client, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "publicface", "display_name": "Public Face", "password": "public-pass-123"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/devices", map[string]string{"name": "Laptop"}, "", api.baseURL)
	device := decodeResponse[Device](t, response)
	start := api.server.now().Add(-time.Minute)
	events := []Event{
		{ID: "event-0000000001", OccurredAt: start, ProjectPath: strings.Repeat("c", 64), ProjectName: "Secret", Source: "vscode", Kind: "focus"},
		{ID: "event-0000000002", OccurredAt: start.Add(30 * time.Second), ProjectPath: strings.Repeat("c", 64), ProjectName: "Secret", Source: "vscode", Kind: "edit"},
	}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable", Events: events}, device.Token, "")
	requireStatus(t, response, http.StatusOK)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	before := decodeResponse[Dashboard](t, response)
	if len(before.Projects) != 1 {
		t.Fatalf("expected one project before hiding it: %#v", before.Projects)
	}
	projectID := before.Projects[0].ID
	response = api.request(t, api.client, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]bool{"is_public": false}, "", "https://attacker.invalid")
	requireStatus(t, response, http.StatusForbidden)
	response = api.request(t, api.client, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]bool{"is_public": false}, "", api.baseURL)
	requireStatus(t, response, http.StatusOK)
	updated := decodeResponse[Project](t, response)
	if updated.IsPublic {
		t.Fatal("project visibility did not change")
	}
	otherJar, _ := cookiejar.New(nil)
	otherClient := &http.Client{Jar: otherJar, Timeout: 5 * time.Second}
	response = api.request(t, otherClient, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "otherface", "display_name": "Other Face", "password": "other-pass-123"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	response = api.request(t, otherClient, http.MethodPatch, "/api/v1/projects/"+projectID, map[string]bool{"is_public": true}, "", api.baseURL)
	requireStatus(t, response, http.StatusNotFound)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/u/publicface", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	profile := decodeResponse[Dashboard](t, response)
	if len(profile.Projects) != 0 || len(profile.Timeline) != 0 || profile.Stats.TotalSeconds != 0 {
		t.Fatalf("private project leaked: %#v", profile)
	}
	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	own := decodeResponse[Dashboard](t, response)
	if len(own.Projects) != 1 || own.Stats.TotalSeconds != 30 {
		t.Fatalf("owner cannot see private project: %#v", own)
	}
}

func TestDashboardUnionsAITimeAndHidesPrivateProjectAI(t *testing.T) {
	api := newTestAPI(t)
	response := api.request(t, api.client, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "aitime", "display_name": "AI Time", "password": "ai-time-pass-123"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/devices", map[string]string{"name": "Laptop"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	device := decodeResponse[Device](t, response)
	start := api.server.now().Add(-10 * time.Minute)
	publicKey, privateKey := strings.Repeat("d", 64), strings.Repeat("e", 64)
	events := []Event{
		{ID: "event-0000000001", OccurredAt: start, ProjectPath: publicKey, ProjectName: "Public AI", Source: "claude-code", Kind: "ai_start", AI: true},
		{ID: "event-0000000002", OccurredAt: start.Add(30 * time.Second), ProjectPath: privateKey, ProjectName: "Private AI", Source: "claude-code", Kind: "ai_start", AI: true},
		{ID: "event-0000000003", OccurredAt: start.Add(60 * time.Second), ProjectPath: publicKey, ProjectName: "Public AI", Source: "claude-code", Kind: "ai_tool", AI: true},
		{ID: "event-0000000004", OccurredAt: start.Add(90 * time.Second), ProjectPath: publicKey, ProjectName: "Public AI", Source: "claude-code", Kind: "ai_end", AI: true},
		{ID: "event-0000000005", OccurredAt: start.Add(120 * time.Second), ProjectPath: privateKey, ProjectName: "Private AI", Source: "claude-code", Kind: "ai_end", AI: true},
	}
	response = api.request(t, api.client, http.MethodPost, "/api/v1/ingest", ingestRequest{DeviceID: "stable", Events: events}, device.Token, "")
	requireStatus(t, response, http.StatusOK)

	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	own := decodeResponse[Dashboard](t, response)
	if own.Stats.AISeconds != 120 {
		t.Fatalf("overlapping AI intervals counted %d seconds, want 120", own.Stats.AISeconds)
	}
	if _, err := api.server.db.Exec(`UPDATE projects SET is_public = 0 WHERE name = 'Private AI'`); err != nil {
		t.Fatal(err)
	}
	response = api.request(t, api.client, http.MethodGet, "/api/v1/u/aitime", nil, "", "")
	requireStatus(t, response, http.StatusOK)
	profile := decodeResponse[Dashboard](t, response)
	if profile.Stats.AISeconds != 90 || len(profile.Projects) != 1 || profile.Projects[0].Name != "Public AI" {
		t.Fatalf("private AI activity leaked into public profile: %#v", profile)
	}
}

func TestLogoutAndAuthentication(t *testing.T) {
	api := newTestAPI(t)
	response := api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	requireStatus(t, response, http.StatusUnauthorized)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "nobody", "password": "incorrect-password"}, "", api.baseURL)
	requireStatus(t, response, http.StatusUnauthorized)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "logout", "display_name": "Logout", "password": "logout-pass-123"}, "", api.baseURL)
	requireStatus(t, response, http.StatusCreated)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/auth/logout", map[string]string{}, "", api.baseURL)
	requireStatus(t, response, http.StatusNoContent)
	response = api.request(t, api.client, http.MethodGet, "/api/v1/dashboard", nil, "", "")
	requireStatus(t, response, http.StatusUnauthorized)
	parsed, _ := url.Parse(api.baseURL)
	response = api.request(t, api.client, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "logout", "password": "logout-pass-123"}, "", parsed.Scheme+"://"+parsed.Host)
	requireStatus(t, response, http.StatusOK)
}

func (api *testAPI) String() string { return fmt.Sprintf("test API at %s", api.baseURL) }
