package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

// Queue is a process-local JSONL queue. Upload acknowledgement is the only
// operation that removes events; a crash after server acceptance therefore
// results in safe server-side duplicates rather than data loss.
type Queue struct {
	mu     sync.Mutex
	path   string
	events []tracking.Event
	ids    map[string]int
}

func OpenQueue(path string) (*Queue, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create queue directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open event queue: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("protect event queue: %w", err)
	}
	queue := &Queue{path: path, ids: make(map[string]int)}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var event tracking.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			file.Close()
			return nil, fmt.Errorf("decode event queue line %d: %w", line, err)
		}
		event.OccurredAt = event.OccurredAt.UTC().Round(0)
		if err := tracking.ValidateEvent(event); err != nil {
			file.Close()
			return nil, fmt.Errorf("invalid event queue line %d: %w", line, err)
		}
		queue.events = append(queue.events, event)
	}
	if err := scanner.Err(); err != nil {
		file.Close()
		return nil, fmt.Errorf("read event queue: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close event queue: %w", err)
	}
	queue.reindex()
	return queue, nil
}

// Enqueue appends and fsyncs an event before making it visible to uploaders.
// The boolean is false for an exact duplicate ID and payload.
func (q *Queue) Enqueue(event tracking.Event) (bool, error) {
	event.OccurredAt = event.OccurredAt.UTC().Round(0)
	if err := tracking.ValidateEvent(event); err != nil {
		return false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if index, exists := q.ids[event.ID]; exists {
		if reflect.DeepEqual(q.events[index], event) {
			return false, nil
		}
		return false, fmt.Errorf("event id was already queued with different contents")
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return false, fmt.Errorf("encode queued event: %w", err)
	}
	encoded = append(encoded, '\n')
	file, err := os.OpenFile(q.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return false, fmt.Errorf("append event queue: %w", err)
	}
	if _, err = file.Write(encoded); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, fmt.Errorf("persist queued event: %w", err)
	}
	q.events = append(q.events, cloneEvent(event))
	q.ids[event.ID] = len(q.events) - 1
	return true, nil
}

// First returns a stable snapshot of at most limit events.
func (q *Queue) First(limit int) []tracking.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	if limit < 0 {
		limit = 0
	}
	if limit > len(q.events) {
		limit = len(q.events)
	}
	result := make([]tracking.Event, limit)
	for i := range result {
		result[i] = cloneEvent(q.events[i])
	}
	return result
}

// Acknowledge removes the first count events after a confirmed full batch.
func (q *Queue) Acknowledge(count int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if count < 0 || count > len(q.events) {
		return fmt.Errorf("invalid queue acknowledgement")
	}
	if count == 0 {
		return nil
	}
	remaining := q.events[count:]
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, event := range remaining {
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("encode event queue: %w", err)
		}
	}
	if err := atomicWriteFile(q.path, buffer.Bytes(), 0o600); err != nil {
		return fmt.Errorf("rewrite acknowledged event queue: %w", err)
	}
	q.events = append([]tracking.Event(nil), remaining...)
	q.reindex()
	return nil
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.events)
}

func (q *Queue) reindex() {
	q.ids = make(map[string]int, len(q.events))
	for i, event := range q.events {
		q.ids[event.ID] = i
	}
}

func cloneEvent(event tracking.Event) tracking.Event {
	if event.ExitCode != nil {
		value := *event.ExitCode
		event.ExitCode = &value
	}
	return event
}
