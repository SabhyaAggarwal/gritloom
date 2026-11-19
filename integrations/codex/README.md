# Codex adapter

`notify.js` accepts Codex's JSON notification argument (or the same JSON on stdin) and maps turn/tool lifecycle types to AI metadata events. The stable Codex `agent-turn-complete` notification produces `ai_end`; runners that also provide `agent-turn-started` or tool notifications produce `ai_start` and `ai_tool`.

Example Codex configuration, replacing the absolute path:

```toml
notify = ["node", "/absolute/path/to/gritloom/integrations/codex/notify.js"]
```

For an explicitly unattended runner, wrap the command so `GRITLOOM_BACKGROUND=1` is set. Background AI is kept separate from foreground activity.

The adapter reads only `cwd`, `model`, and notification `type`. In particular it does not read or forward Codex `input` messages, prompts, tool arguments/output, source contents, or paths to transcripts. The payload is capped at 256 KiB when read from stdin and is never logged. Codex installations exposing only turn-complete notifications cannot provide a precise start timestamp; editor focus/edit/save signals remain the primary engaged-time source.
