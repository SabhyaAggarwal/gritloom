#!/usr/bin/env node
"use strict";

const { readHookJSON, sendEvent } = require("../agent-client");

function kindFor(type) {
  switch (type) {
    case "turn-started":
    case "agent-turn-started":
      return "ai_start";
    case "tool-call":
    case "tool-complete":
    case "agent-tool-complete":
      return "ai_tool";
    case "agent-turn-complete":
    case "turn-complete":
      return "ai_end";
    default:
      return "";
  }
}

async function main() {
  // Codex notify passes one JSON argument. Also accept stdin for hook runners
  // that use that convention. The payload is never logged or forwarded.
  let notification = null;
  const argument = process.argv[2];
  if (argument && argument.trim().startsWith("{")) {
    try {
      notification = JSON.parse(argument);
    } catch (_) {
      return;
    }
  } else if (!process.stdin.isTTY) {
    notification = await readHookJSON();
  }
  if (!notification || typeof notification.cwd !== "string" || !notification.cwd) {
    return;
  }
  const type = typeof notification.type === "string" ? notification.type : "agent-turn-complete";
  const kind = kindFor(type);
  if (!kind) {
    return;
  }
  await sendEvent({
    project_path: notification.cwd,
    source: "codex",
    kind,
    ai: true,
    background: process.argv.includes("--background") || process.env.GRITLOOM_BACKGROUND === "1",
    model: notification.model
  });
}

main().catch(() => {}).finally(() => {
  process.exitCode = 0;
});
