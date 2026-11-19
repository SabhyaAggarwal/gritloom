# GritLoom local integrations

These dependency-free Node.js adapters send metadata only to the local agent at `127.0.0.1:47653`; none contacts the GritLoom server directly. The agent owns project hashing, git/worktree inference, exclusions, pause state, offline persistence, and authenticated batched upload.

- `claude-code/`: Claude prompt/tool lifecycle hooks.
- `codex/`: Codex turn/tool notification adapter.
- `generic/`: a documented generic sender and metadata-only git post-commit hook.

Every adapter reads `local_token` from `gritloom/agent.json` in the platform user config directory. Agent connection failures are intentionally silent so AI tools and git hooks continue normally. No adapter logs or persists its input payload.

Prompt text, source contents, edit contents, keystrokes, tool input/output, terminal output, and transcript paths are never uploaded. Raw local project paths are sent only over the bearer-authenticated loopback API and are replaced with lowercase SHA-256 project identifiers before the agent's JSONL queue is written.
