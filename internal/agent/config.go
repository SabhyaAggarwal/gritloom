package agent

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	ApplicationDir = "gritloom"
	DefaultAddress = "127.0.0.1:47653"
)

// Config contains local secrets and control settings. It is always written
// with mode 0600. ExcludedProjects remains local and is never uploaded.
type Config struct {
	Server           string     `json:"server"`
	DeviceToken      string     `json:"device_token"`
	DeviceID         string     `json:"device_id"`
	LocalToken       string     `json:"local_token"`
	PausedUntil      *time.Time `json:"paused_until,omitempty"`
	ExcludedProjects []string   `json:"excluded_projects,omitempty"`
}

func DefaultConfigPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(directory, ApplicationDir, "agent.json"), nil
}

func DefaultQueuePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "events.jsonl")
}

func LoadConfig(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("read agent config: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("decode agent config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return config, fmt.Errorf("protect agent config: %w", err)
	}
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// Setup creates or updates configuration while preserving stable random IDs.
// It does not contact the server, so setup and offline use are independent.
func Setup(path, server, deviceToken string, excluded []string) (Config, error) {
	var config Config
	if existing, err := LoadConfig(path); err == nil {
		config = existing
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	normalizedServer, err := normalizeServer(server)
	if err != nil {
		return Config{}, err
	}
	if err := validateSecret(deviceToken, "device token"); err != nil {
		return Config{}, err
	}
	if config.DeviceID == "" {
		config.DeviceID, err = randomHex(16)
		if err != nil {
			return Config{}, err
		}
	}
	if config.LocalToken == "" {
		bytes, randomErr := randomBytes(32)
		if randomErr != nil {
			return Config{}, fmt.Errorf("generate local bearer token: %w", randomErr)
		}
		config.LocalToken = base64.RawURLEncoding.EncodeToString(bytes)
	}
	config.Server = normalizedServer
	config.DeviceToken = deviceToken
	if excluded != nil {
		config.ExcludedProjects = nil
		for _, excludedPath := range excluded {
			canonical, canonicalErr := canonicalLocalPath(excludedPath)
			if canonicalErr != nil {
				return Config{}, fmt.Errorf("invalid excluded project path: %w", canonicalErr)
			}
			config.ExcludedProjects = append(config.ExcludedProjects, canonical)
		}
	}
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	if err := SaveConfig(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func SaveConfig(path string, config Config) error {
	if err := validateConfig(config); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent config: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write agent config: %w", err)
	}
	return nil
}

func (c Config) Paused(now time.Time) bool {
	return c.PausedUntil != nil && now.Before(*c.PausedUntil)
}

func validateConfig(config Config) error {
	if _, err := normalizeServer(config.Server); err != nil {
		return err
	}
	if err := validateSecret(config.DeviceToken, "device token"); err != nil {
		return err
	}
	if len(config.DeviceID) == 0 || len(config.DeviceID) > 128 || strings.IndexFunc(config.DeviceID, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid device_id")
	}
	if err := validateSecret(config.LocalToken, "local token"); err != nil {
		return err
	}
	if config.PausedUntil != nil && config.PausedUntil.IsZero() {
		return fmt.Errorf("invalid paused_until")
	}
	return nil
}

func validateSecret(value, name string) error {
	if value == "" || len(value) > 1024 || strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid %s", name)
	}
	return nil
}

func normalizeServer(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("server must be an http(s) URL without credentials, query, or fragment")
	}
	if u.Scheme == "http" {
		host := u.Hostname()
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			return "", fmt.Errorf("remote servers must use HTTPS")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/"), nil
}

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func randomHex(size int) (string, error) {
	value, err := randomBytes(size)
	if err != nil {
		return "", fmt.Errorf("generate random identifier: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gritloom-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return nil
}
