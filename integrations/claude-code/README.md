# Claude Code adapter

`hook.js` converts Claude Code hook timing into metadata-only `ai_start`, `ai_tool`, and `ai_end` events. It never forwards prompt text, tool input/output, commands, source content, or transcript paths. Hook input is capped at 256 KiB and discarded. An unavailable or paused local agent produces no hook error.

Add commands like these to Claude Code's `settings.json`, replacing `/absolute/path/to/gritloom` with the checkout/install location:

```json
{
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "node /absolute/path/to/gritloom/integrations/claude-code/hook.js user-prompt"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "node /absolute/path/to/gritloom/integrations/claude-code/hook.js pre-tool"}]}],
    "PostToolUse": [{"hooks": [{"type": "command", "command": "node /absolute/path/to/gritloom/integrations/claude-code/hook.js post-tool"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "node /absolute/path/to/gritloom/integrations/claude-code/hook.js stop"}]}]
  }
}
```

For an explicitly unattended invocation, append `--background` to each command (or set `GRITLOOM_BACKGROUND=1`). Background AI is accounted separately from engaged editor work. Interactive hooks should not use this flag.

The hook sends only the workspace `cwd`, model identifier when available, timestamp, random ID, source, kind, and AI/background flags to the bearer-authenticated loopback agent. The agent hashes the project path before its persistent queue and server upload.
