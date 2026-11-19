# GritLoom local agent

The agent uses only the Go standard library. It watches no files and installs no OS service, keylogger, process monitor, or native idle collector; editor, AI harness, task/debug, and git-hook integrations provide the activity signals.

```sh
gritloom-agent setup --server https://gritloom.example --token DEVICE_TOKEN
gritloom-agent run
gritloom-agent status
gritloom-agent pause --duration 1h
gritloom-agent resume
```

Repeat `--exclude /local/project/path` during setup for every project that must not be tracked. Exclusions apply to the project and its children. They are canonicalized and stored only in the local config. Setup preserves the stable random device ID, local bearer token, and existing exclusions when rerun without `--exclude`.

## Local files and endpoints

`os.UserConfigDir()` selects the platform config directory. On Linux this is normally `~/.config/gritloom`:

- `agent.json` (mode `0600`): server/device credentials, random local token, pause state, and exclusions.
- `events.jsonl` (mode `0600`): privacy-safe pending events. A batch is removed only after the server acknowledges every item as accepted or duplicate.

The agent listens only on `127.0.0.1:47653` and requires `Authorization: Bearer LOCAL_TOKEN`. All requests with an `Origin` header or cross-site browser fetch metadata are rejected, as are non-loopback `Host` headers.

- `POST /v1/events`: metadata event with a raw local `project_path`; the agent immediately resolves and hashes it before queueing.
- `GET /v1/status`: pause, queue, and upload status without exposing credentials.
- `POST /v1/pause`: optional JSON `{"duration":"1h"}` (default one hour).
- `POST /v1/resume`: clear the pause.

The uploader sends batches of 100 events to `{server}/api/v1/ingest` using the device token. It retries retained events every 30 seconds. Pausing stops new collection but does not prevent already queued events from uploading.

## Privacy and accounting

Uploaded `project_path` values are exactly lowercase 64-character SHA-256 hashes. Ordinary and linked-worktree git roots are detected without executing git; the project name, branch, and sanitized HTTP(S) remote are inferred locally. Files are clean repository-relative paths. Event IDs and timestamps use the server's accepted bounds.

The pure `internal/tracking` engine sorts out-of-order events, unions overlapping sources into one engaged timeline, breaks foreground gaps longer than two minutes, accounts unattended background AI separately, and retains metadata-only evidence summaries. Prompt text, source contents, edit contents, keystrokes, tool payloads, and absolute local paths are never placed in the upload queue.
