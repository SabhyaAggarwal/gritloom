// Package live provides per-user in-memory SSE fan-out.
package live

import "sync"

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(userID string) (<-chan []byte, func()) {
	channel := make(chan []byte, 4)
	h.mu.Lock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[chan []byte]struct{})
	}
	h.subscribers[userID][channel] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if subscribers := h.subscribers[userID]; subscribers != nil {
				if _, ok := subscribers[channel]; ok {
					delete(subscribers, channel)
					close(channel)
				}
				if len(subscribers) == 0 {
					delete(h.subscribers, userID)
				}
			}
			h.mu.Unlock()
		})
	}
	return channel, cancel
}

func (h *Hub) Publish(userID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for channel := range h.subscribers[userID] {
		select {
		case channel <- payload:
		default:
			// Slow clients can use the next complete dashboard snapshot.
		}
	}
}
