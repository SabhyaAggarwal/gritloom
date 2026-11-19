package agent

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

func NewHandler(agent *Agent) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/events", agent.handleEvents)
	mux.HandleFunc("/v1/status", agent.handleStatus)
	mux.HandleFunc("/v1/pause", agent.handlePause)
	mux.HandleFunc("/v1/resume", agent.handleResume)
	return agent.protectLocal(mux)
}

func (a *Agent) protectLocal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Header.Get("Origin") != "" || request.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeLocalError(writer, http.StatusForbidden, "browser-origin requests are rejected")
			return
		}
		if !loopbackHost(request.Host) {
			writeLocalError(writer, http.StatusForbidden, "loopback Host header required")
			return
		}
		header := request.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.localToken())) != 1 {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="gritloom-local"`)
			writeLocalError(writer, http.StatusUnauthorized, "valid bearer token required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (a *Agent) handleEvents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		localMethodNotAllowed(writer, http.MethodPost)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 128*1024)
	var raw tracking.Event
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeLocalError(writer, http.StatusBadRequest, "invalid metadata-only event JSON")
		return
	}
	event, queued, err := a.Enqueue(raw)
	if errors.Is(err, ErrPaused) {
		writeLocalError(writer, http.StatusConflict, ErrPaused.Error())
		return
	}
	if errors.Is(err, ErrExcluded) {
		writeLocalJSON(writer, http.StatusAccepted, map[string]any{"queued": false, "excluded": true})
		return
	}
	if err != nil {
		writeLocalError(writer, http.StatusBadRequest, "event failed privacy or metadata validation")
		return
	}
	writeLocalJSON(writer, http.StatusAccepted, map[string]any{"queued": queued, "id": event.ID})
}

func (a *Agent) handleStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		localMethodNotAllowed(writer, http.MethodGet)
		return
	}
	writeLocalJSON(writer, http.StatusOK, a.Status())
}

func (a *Agent) handlePause(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		localMethodNotAllowed(writer, http.MethodPost)
		return
	}
	var input struct {
		Duration string `json:"duration"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil && err != io.EOF {
		writeLocalError(writer, http.StatusBadRequest, "invalid pause request")
		return
	} else if err == nil && decoder.Decode(&struct{}{}) != io.EOF {
		writeLocalError(writer, http.StatusBadRequest, "invalid pause request")
		return
	}
	duration := time.Hour
	var err error
	if input.Duration != "" {
		duration, err = time.ParseDuration(input.Duration)
		if err != nil || duration <= 0 {
			writeLocalError(writer, http.StatusBadRequest, "duration must be a positive Go duration such as 1h")
			return
		}
	}
	if err := a.Pause(duration); err != nil {
		writeLocalError(writer, http.StatusInternalServerError, "could not persist pause")
		return
	}
	writeLocalJSON(writer, http.StatusOK, a.Status())
}

func (a *Agent) handleResume(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		localMethodNotAllowed(writer, http.MethodPost)
		return
	}
	if err := a.Resume(); err != nil {
		writeLocalError(writer, http.StatusInternalServerError, "could not persist resume")
		return
	}
	writeLocalJSON(writer, http.StatusOK, a.Status())
}

func loopbackHost(hostport string) bool {
	host := hostport
	if parsedHost, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsedHost
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func localMethodNotAllowed(writer http.ResponseWriter, method string) {
	writer.Header().Set("Allow", method)
	writeLocalError(writer, http.StatusMethodNotAllowed, "method not allowed")
}

func writeLocalError(writer http.ResponseWriter, status int, message string) {
	writeLocalJSON(writer, status, map[string]string{"error": message})
}

func writeLocalJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
