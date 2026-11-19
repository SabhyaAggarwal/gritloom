# Accounting, architecture, and trust model

## Data flow

1. An editor extension, AI-harness hook, generic integration, or git hook emits a metadata-only event.
2. The integration sends the event over a bearer-authenticated loopback connection to `127.0.0.1:47653`.
3. The local agent checks pause state and exclusions, resolves the git root/worktree, hashes the project identity, converts file names to repository-relative paths, and discards the raw local path.
4. The agent appends the privacy-safe event to a mode-`0600` JSONL queue. Upload batches contain at most 100 events and are removed only after the server accepts or confirms every item.
5. The server authenticates the device token, validates and bounds the batch, stores events idempotently, and reconstructs that user’s sessions with the shared tracking engine.
6. The dashboard updates over server-sent events. If streaming fails, the browser falls back to a 30-second refresh.
7. A submission copies selected engaged sessions and their evidence into immutable snapshot rows. Later activity and visibility changes do not rewrite those snapshots.

The server and agent use `internal/tracking` for interval accounting. The server adds reviewer labels, session identifiers, and evidence hashes after those intervals are built.

## Engaged intervals

- Only foreground events participate in the engaged timeline.
- Two adjacent foreground events form an interval only when they belong to the same project and are at most two minutes apart.
- Events from multiple sources are sorted and overlapping intervals merged. Two tools running at once do not double the time.
- A project switch breaks attribution: the gap before the switch is unknown and is not credited to the previous project.
- A single event has no closing bound and therefore contributes zero seconds.
- Background AI events do not close or interrupt a foreground interval.

The two-minute rule is deliberately conservative. A focused heartbeat from the editor arrives at most every 30 seconds while focus remains active. Thinking without any observable activity for more than two minutes is not automatically counted.

## AI intervals

An event is background AI only when all of these are true:

- `ai` is true;
- `background` is true; and
- its kind is `ai_start`, `ai_tool`, or `ai_end`.

Background AI is tracked per project with a 15-minute maximum gap. Independent background jobs in different projects can overlap and are displayed separately from engaged time. They cannot be submitted as engaged sessions.

For the AI-assisted engaged statistic, non-background AI lifecycle events form same-project intervals with the two-minute bound. Those intervals are unioned across projects before overlap with the selected day is calculated. This statistic is not total AI runtime and is not calculated by subtracting background AI from engaged time.

## Evidence labels

Labels summarize event diversity; they are not grades or fraud verdicts.

- **Observed locally:** the session has one kind of signal or only weak corroboration.
- **Corroborated:** at least two signal classes are present among human edits, AI lifecycle events, verification activity, and milestones.
- **Review-ready:** the session is corroborated, contains an edit/save, and also contains a commit or a passing verification event.

Failed tests remain visible. They are evidence of iteration, not a penalty. `review_ready` means the record has a useful trail for a person to inspect, not that GritLoom has approved the work.

## Evidence and manifest hashes

Each session has an evidence hash. During reconstruction, sessions are chronological and each hash includes the preceding session hash plus canonical metadata lines for its evidence events. A selected snapshot can inherit a preceding hash from an unselected session, so the session hash is primarily a tamper-evident identifier inside the full record.

A submission has a separate versioned JSON manifest covering its ID, author, creation timestamp, note, and complete selected snapshots. The raw manifest endpoint serves the exact bytes hashed into `manifest_hash`. This is the portable external verification boundary; see [reviewer-format.md](reviewer-format.md).

## What the system can and cannot prove

The design can show that:

- a device token accepted by the account submitted particular metadata;
- events were reconstructed using deterministic, conservative rules;
- a submission contains only its selected session snapshots;
- those snapshots and the note have not changed since the manifest was created.

The design cannot prove that:

- a local integration accurately described a person’s attention or intent;
- a device token or client was not deliberately used to fabricate events;
- AI assistance was declared by every possible harness;
- an hour of observed activity is an hour of productive struggle;
- a reviewer should approve a project.

Program authors must combine the evidence with repository history, shipped artifacts, builder context, and their own review. GritLoom intentionally avoids a pseudo-objective “struggle score.”

## Cost and scaling model

The deployment unit is one Go process, one SQLite database in WAL mode, static frontend assets, and an attached volume. There is no required managed database, queue, CDN, analytics service, or per-request AI API. The local agent has no third-party Go dependencies and does not run a keylogger, screenshot service, process monitor, or file watcher.

This keeps the baseline hosting footprint small, but the bounty’s target requires measurement rather than extrapolation. In particular, ingestion currently reloads and rebuilds the uploading user’s full timeline. That is simple and correct for an MVP, but it becomes expensive for long histories. Before full-scale adoption:

1. benchmark realistic event rates and two-year histories;
2. make timeline reconstruction incremental while preserving out-of-order correction and immutable submissions;
3. add retention/export policies agreed with users;
4. measure database size, disk IOPS, backup time, SSE connections, and p95/p99 ingestion latency;
5. set resource limits and alerts from observed data.

SQLite must remain on a local or safely shared filesystem with one writer-aware application instance. Do not run multiple replicas against a volume that does not support SQLite’s locking requirements. SSE fan-out is in-memory, so multi-replica deployment also needs a shared update mechanism.
