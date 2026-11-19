package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type attempt struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]attempt
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{attempts: make(map[string]attempt)}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.attempts) > 10000 {
		for candidate, value := range l.attempts {
			if now.After(value.reset) {
				delete(l.attempts, candidate)
			}
		}
		if len(l.attempts) > 10000 {
			l.attempts = make(map[string]attempt)
		}
	}
	value := l.attempts[key]
	if now.After(value.reset) {
		value = attempt{reset: now.Add(window)}
	}
	if value.count >= limit {
		return false
	}
	value.count++
	l.attempts[key] = value
	return true
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]; forwarded != "" {
			if ip := net.ParseIP(strings.TrimSpace(forwarded)); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	if ip := net.ParseIP(r.RemoteAddr); ip != nil {
		return ip.String()
	}
	return "unknown"
}
