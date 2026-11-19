package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

var ErrPaused = errors.New("tracking is paused")

// Event and RawEvent make the local API convenient for integrations and
// tests. RawEvent carries a local path only until Resolver converts it.
type Event = tracking.Event
type RawEvent = tracking.Event

// Status is safe to return to local plugins; it contains neither bearer token
// nor excluded/raw project paths.
type Status struct {
	Running         bool       `json:"running"`
	Paused          bool       `json:"paused"`
	PausedUntil     *time.Time `json:"paused_until,omitempty"`
	Queued          int        `json:"queued"`
	DeviceID        string     `json:"device_id"`
	Server          string     `json:"server"`
	LastUploadAt    *time.Time `json:"last_upload_at,omitempty"`
	LastUploadError string     `json:"last_upload_error,omitempty"`
	Accepted        int        `json:"accepted"`
	Duplicates      int        `json:"duplicates"`
}

type Agent struct {
	mu         sync.RWMutex
	uploadMu   sync.Mutex
	config     Config
	configPath string
	queue      *Queue
	resolver   *Resolver
	now        func() time.Time
	running    bool
	lastUpload *time.Time
	lastError  string
	accepted   int
	duplicates int
}

func Open(configPath string) (*Agent, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	resolver, err := NewResolver(config.ExcludedProjects)
	if err != nil {
		return nil, err
	}
	queue, err := OpenQueue(DefaultQueuePath(configPath))
	if err != nil {
		return nil, err
	}
	return &Agent{config: config, configPath: configPath, queue: queue, resolver: resolver, now: time.Now}, nil
}

// Enqueue converts and durably queues one event. The pause check and append
// share a read lock so a pause cannot race with an event slipping in after it.
func (a *Agent) Enqueue(raw tracking.Event) (tracking.Event, bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	now := a.now()
	if a.config.Paused(now) {
		return tracking.Event{}, false, ErrPaused
	}
	event, err := a.resolver.ResolveEvent(raw, now)
	if err != nil {
		return tracking.Event{}, false, err
	}
	queued, err := a.queue.Enqueue(event)
	if err != nil {
		return tracking.Event{}, false, err
	}
	return event, queued, nil
}

func (a *Agent) Pause(duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("pause duration must be positive")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	config := a.config
	until := a.now().UTC().Add(duration)
	config.PausedUntil = &until
	if err := SaveConfig(a.configPath, config); err != nil {
		return err
	}
	a.config = config
	return nil
}

func (a *Agent) Resume() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	config := a.config
	config.PausedUntil = nil
	if err := SaveConfig(a.configPath, config); err != nil {
		return err
	}
	a.config = config
	return nil
}

func (a *Agent) Status() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return Status{
		Running: a.running, Paused: a.config.Paused(a.now()), PausedUntil: cloneTime(a.config.PausedUntil),
		Queued: a.queue.Len(), DeviceID: a.config.DeviceID, Server: a.config.Server,
		LastUploadAt: cloneTime(a.lastUpload), LastUploadError: a.lastError,
		Accepted: a.accepted, Duplicates: a.duplicates,
	}
}

// Flush uploads at most one server-compatible batch. Events are acknowledged
// locally only when every event was accepted or confirmed as a duplicate.
func (a *Agent) Flush(ctx context.Context) error {
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	batch := a.queue.First(100)
	if len(batch) == 0 {
		return nil
	}
	a.mu.RLock()
	config := a.config
	a.mu.RUnlock()
	accepted, duplicates, err := uploadBatch(ctx, config, batch)
	if err == nil && accepted+duplicates != len(batch) {
		err = fmt.Errorf("server acknowledged %d of %d events", accepted+duplicates, len(batch))
	}
	if err == nil {
		err = a.queue.Acknowledge(len(batch))
	}
	a.mu.Lock()
	if err != nil {
		a.lastError = err.Error()
	} else {
		now := a.now().UTC()
		a.lastUpload = &now
		a.lastError = ""
		a.accepted += accepted
		a.duplicates += duplicates
	}
	a.mu.Unlock()
	return err
}

// Run serves the loopback API and retries pending batches until cancellation.
func (a *Agent) Run(ctx context.Context, address string) error {
	listener, err := listenLoopback(address)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: NewHandler(a), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	serverError := make(chan error, 1)
	go func() { serverError <- server.Serve(listener) }()
	go a.uploadLoop(runCtx)
	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-serverError:
		cancel()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *Agent) uploadLoop(ctx context.Context) {
	flush := func() {
		flushCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		_ = a.Flush(flushCtx) // The error is exposed through /v1/status.
	}
	flush()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			flush()
		}
	}
}

func (a *Agent) localToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.LocalToken
}

func listenLoopback(address string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid local listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("local API must listen on a loopback IP")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for local plugins: %w", err)
	}
	return listener, nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
