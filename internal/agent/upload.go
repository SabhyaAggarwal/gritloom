package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

type ingestRequest struct {
	DeviceID string           `json:"device_id"`
	Events   []tracking.Event `json:"events"`
}

func uploadBatch(ctx context.Context, config Config, events []tracking.Event) (accepted, duplicates int, returnErr error) {
	encoded, err := json.Marshal(ingestRequest{DeviceID: config.DeviceID, Events: events})
	if err != nil {
		return 0, 0, fmt.Errorf("encode ingest batch: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Server+"/api/v1/ingest", bytes.NewReader(encoded))
	if err != nil {
		return 0, 0, fmt.Errorf("create ingest request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+config.DeviceToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "gritloom-agent/0.1")
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return 0, 0, fmt.Errorf("upload event batch: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		if closeErr := response.Body.Close(); returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close ingest response: %w", closeErr)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("server ingest returned %s", response.Status)
	}
	var result struct {
		Accepted   int `json:"accepted"`
		Duplicates int `json:"duplicates"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64*1024))
	if err := decoder.Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("decode ingest response: %w", err)
	}
	if result.Accepted < 0 || result.Duplicates < 0 {
		return 0, 0, fmt.Errorf("server returned invalid ingest counts")
	}
	return result.Accepted, result.Duplicates, nil
}
