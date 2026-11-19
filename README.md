# GritLoom

**Your work, woven with context.**

GritLoom is an open-source, privacy-conscious activity tracker for people building projects with AI. A small local agent turns editor, AI-harness, test, debug, and git signals into conservative engaged-time sessions. A centralized dashboard helps builders reflect on their habits and deliberately share immutable evidence snapshots with reviewers.

GritLoom does **not** claim that an algorithm can certify “productive struggle.” It records the time between real activity signals and shows the edits, failed and passing tests, AI assistance, revisions, commits, and builder’s note behind selected hours. A human reviewer can judge the work with that context.

## Why it is useful before a submission

- See engaged time today, this week, by project, and across a seven-day chart.
- Keep AI-assisted engaged time and unattended background AI time distinct.
- Notice iterations rather than just hours: edits, tests, failures, and milestones stay attached to sessions.
- Choose a weekly rhythm, track a making streak, and keep a public-by-default project profile—or make the profile or any project private.
- Use the explicit `?demo=1` workspace without an account. Sample data never substitutes for unavailable real data.

## What is included

- **Local agent:** standard-library Go, authenticated loopback API, pause/resume, persistent exclusions, durable offline queue, batched upload, and privacy-safe project hashing.
- **Integrations:** VS Code-compatible editor extension, Claude Code hooks, Codex notifications, generic event sender, and git post-commit hook.
- **Server:** Go and SQLite (WAL), accounts, bcrypt password hashes, opaque hashed session/device tokens, projects, device revocation, real-time SSE dashboards, public profiles, and bounded event ingestion.
- **Reviewer workflow:** selected engaged sessions become immutable snapshots with granular evidence, a SHA-256 manifest, an unguessable read-only link, and a raw manifest download for independent verification.
- **Web UI:** responsive React/TypeScript dashboard, accessible dialogs and forms, mobile navigation, public profiles, and reviewer views. All runtime assets are local; no third-party font, analytics, or CDN requests.

## Quick start

Prerequisites: Go 1.26.11 or newer and Node.js 18 or newer. The Go patch version matters because older standard libraries have published security advisories.

```sh
npm --prefix web ci
npm --prefix web run build
go run ./cmd/server
```

Open `http://localhost:8080`, or explore `http://localhost:8080/?demo=1`. The server creates `data/gritloom.db`. For a different location, set `GRITLOOM_DATA_DIR`.

For local UI development, run the server as above and Vite separately:

```sh
npm --prefix web run dev
```

Vite listens on port 5173 and proxies `/api` to port 8080.

## Connect a device

1. Create an account and open **Settings & privacy → Devices & agent access**.
2. Generate a device token. It is shown once; only its hash is stored server-side.
3. Build and configure the agent:

   ```sh
   go build -o gritloom-agent ./cmd/agent
   ./gritloom-agent setup --server http://localhost:8080 --token DEVICE_TOKEN
   ./gritloom-agent run
   ```

   Remote servers must use HTTPS. Plain HTTP is accepted only for loopback development.

4. Install the editor and AI-harness integrations you use:
   - [VS Code, Cursor, Windsurf, and VSCodium](extensions/vscode/README.md)
   - [Claude Code, Codex, generic integrations, and git hooks](integrations/README.md)
5. Pause at any time:

   ```sh
   ./gritloom-agent pause --duration 1h
   ./gritloom-agent resume
   ./gritloom-agent status
   ```

   Repeat `--exclude /local/project/path` during setup for projects that must never be tracked. Exclusions and raw local paths stay on the device. Pausing stops new collection; already queued events can still upload.

See [the agent guide](cmd/agent/README.md) for queue and loopback security details.

## How accounting works

The local and server implementations share the same interval engine.

- An engaged interval must be bounded by two foreground signals from the same project, no more than **two minutes** apart.
- Overlapping editor and harness signals are unioned, not added together.
- The unknown gap across a project switch is not assigned to either project.
- Idle gaps and isolated events do not invent a session.
- Background AI has a separate per-project timeline with a **15-minute** continuity bound. It never becomes engaged time.
- AI-assisted engaged intervals are unioned separately for the AI statistic; background AI is not subtracted from them.
- Durations are summed before being rounded down to seconds.

This is intentionally conservative. It estimates observed engaged-building time, not attention, effort, learning quality, or the value of a project. Read [the accounting and trust model](docs/architecture.md) before relying on it for review.

## Privacy boundary

| Data | Local queue and upload | Public profile | Reviewer snapshot |
| --- | --- | --- | --- |
| Event time, source, kind, project identifier | Yes | Public-project sessions only | Selected sessions only |
| Project name and sanitized repository URL | Yes | Public projects only | Project name/color only; no repository URL |
| Branch | Yes | No | Selected evidence events only |
| Repository-relative file path, change counts, test exit code | Yes | No granular events | Selected sessions only |
| AI model name and foreground/background marker | Yes | Aggregate AI time only | Selected sessions only |
| Source text, edit contents, prompts, tool payloads | **Never** | **Never** | **Never** |
| Screenshots, keystrokes, terminal output, transcripts | **Never** | **Never** | **Never** |
| Absolute local paths and exclusion list | **Never uploaded** | **Never** | **Never** |

Profiles and newly created projects are public by default. A private profile exposes nothing; a private project is removed from public totals, timelines, charts, and AI statistics while remaining in the owner’s dashboard. Visibility changes do not alter reviewer snapshots already created. Anyone with a reviewer token can read that snapshot, so share links deliberately.

Read [the security guide](docs/security.md) for deployment boundaries and limitations.

## Independent review

A reviewer can inspect the timeline in a browser and download the raw manifest. To verify its integrity without trusting the UI:

```sh
curl -fsS https://gritloom.example/api/v1/submissions/REVIEW_TOKEN/manifest -o manifest.json
curl -fsS https://gritloom.example/api/v1/submissions/REVIEW_TOKEN -o submission.json
sha256sum manifest.json
```

The digest must equal `manifest_hash` in `submission.json` and the `X-Gritloom-Manifest-SHA256` response header on the raw manifest endpoint. The endpoint serves the exact bytes covered by the hash—no trailing newline is added.

This proves that the note and selected snapshots have not changed since submission. It does **not** prove that a client told the truth, that an AI wrote particular code, or that the work qualifies for a program. Those remain review decisions. See [the manifest format](docs/reviewer-format.md).

## Docker

Build the self-contained image:

```sh
docker build -t gritloom .
```

Or use Compose for local evaluation:

```sh
docker compose up --build
```

Compose binds to `127.0.0.1:8080`, uses an unprivileged container, drops capabilities, and persists SQLite in the `gritloom-data` volume. Its default development cookie settings are for local HTTP only. Follow [the deployment guide](deploy/README.md) before exposing a server to the internet.

## Verification

```sh
go test -count=1 ./...
go test -race -count=1 ./internal/tracking ./internal/agent ./internal/server
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
npm --prefix web run build
npm --prefix web audit
```

The integration suite covers registration, secure cookies and headers, ingestion idempotency and conflicts, conservative accounting, project/privacy boundaries, cross-user authorization, device revocation, immutable submissions, and raw manifest verification. Browser checks cover desktop/mobile rendering, submission interaction, real registration, project visibility, and device lifecycle.

The agent cross-builds for Windows, macOS, and Linux without CGO. Cross-compiled binaries are not a substitute for running integrations on each operating system; see the current limitations below.

## Bounty scope and current limits

GritLoom implements the technical foundation requested by the bounty: AGPLv3, passive tracking, a central real-time service, public profiles, project and git metadata, privacy controls, extensible AI integrations, reviewer evidence, and a low-cost single-binary deployment.

It is not honest to claim the bounty is already won:

- The acceptance criterion of tracking **1,000 weighted Hack Club projects** requires real adoption and cannot be completed by code.
- Productive-struggle approval remains human judgment, not an anti-fraud score.
- The popular integrations are included, but “all models and harnesses” is achieved through the generic plugin protocol as new adapters are written—not by claiming every harness has a native adapter today.
- No independent security audit or target-scale production load test has been performed. A hosting cost under $500/month is an architectural goal, not a measured guarantee.
- Windows and macOS binaries can be cross-compiled, but those builds still need on-device validation. The editor extension has not been published to a marketplace.
- The current server rebuilds a user’s timeline on ingestion and is intended for a single SQLite-backed instance. Benchmark and optimize it before accepting the bounty’s full-scale load.

## License

GritLoom is available under the [GNU Affero General Public License v3.0](LICENSE). If you operate a modified version as a network service, you must offer its corresponding source to users. The web interface includes a source link for that purpose.
