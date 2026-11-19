package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/agent"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gritloom-agent:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		printUsage()
		return fmt.Errorf("a command is required")
	}
	configPath, err := agent.DefaultConfigPath()
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "setup":
		flags := flag.NewFlagSet("setup", flag.ContinueOnError)
		server := flags.String("server", "", "GritLoom server URL")
		token := flags.String("token", "", "device bearer token")
		var excluded stringList
		flags.Var(&excluded, "exclude", "local project path to exclude (repeatable)")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		config, err := agent.Setup(configPath, *server, *token, excluded)
		if err != nil {
			return err
		}
		fmt.Printf("GritLoom agent configured for %s (device %s).\n", config.Server, config.DeviceID)
		return nil
	case "run":
		flags := flag.NewFlagSet("run", flag.ContinueOnError)
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		running, err := agent.Open(configPath)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Fprintln(os.Stderr, "GritLoom local API listening on http://"+agent.DefaultAddress)
		return running.Run(ctx, agent.DefaultAddress)
	case "status":
		flags := flag.NewFlagSet("status", flag.ContinueOnError)
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		return showStatus(configPath)
	case "pause":
		flags := flag.NewFlagSet("pause", flag.ContinueOnError)
		duration := flags.Duration("duration", time.Hour, "pause duration")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if *duration <= 0 {
			return fmt.Errorf("pause duration must be positive")
		}
		return changePause(configPath, *duration)
	case "resume":
		flags := flag.NewFlagSet("resume", flag.ContinueOnError)
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		return changePause(configPath, 0)
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func showStatus(configPath string) error {
	config, err := agent.LoadConfig(configPath)
	if err != nil {
		return err
	}
	var status agent.Status
	if err := localControl(config, http.MethodGet, "/v1/status", nil, &status); err == nil {
		return json.NewEncoder(os.Stdout).Encode(status)
	}
	opened, err := agent.Open(configPath)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(opened.Status())
}

func changePause(configPath string, duration time.Duration) error {
	config, err := agent.LoadConfig(configPath)
	if err != nil {
		return err
	}
	path := "/v1/pause"
	var body any
	if duration == 0 {
		path = "/v1/resume"
	} else {
		body = map[string]string{"duration": duration.String()}
	}
	var status agent.Status
	if err := localControl(config, http.MethodPost, path, body, &status); err == nil {
		return json.NewEncoder(os.Stdout).Encode(status)
	}
	// Pause and resume must also work before the daemon is started.
	opened, err := agent.Open(configPath)
	if err != nil {
		return err
	}
	if duration == 0 {
		err = opened.Resume()
	} else {
		err = opened.Pause(duration)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(opened.Status())
}

func localControl(config agent.Config, method, path string, body, result any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, "http://"+agent.DefaultAddress+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+config.LocalToken)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("local agent returned %s", response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(result)
}

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	if value == "" {
		return fmt.Errorf("excluded path cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage:
  gritloom-agent setup --server URL --token TOKEN [--exclude PATH ...]
  gritloom-agent run
  gritloom-agent status
  gritloom-agent pause --duration 1h
  gritloom-agent resume`)
}
