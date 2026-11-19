#!/usr/bin/env node
"use strict";

const { sendEvent } = require("../agent-client");

const kinds = new Set(["focus", "edit", "save", "ai_start", "ai_end", "ai_tool", "build", "test", "debug", "commit"]);

function integer(value) {
  if (!/^-?\d+$/.test(value || "")) {
    return null;
  }
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) ? parsed : null;
}

async function main() {
  const args = process.argv.slice(2);
  const event = { source: "generic", additions: 0, deletions: 0, ai: false, background: false };
  const values = {
    "--project": "project_path",
    "--kind": "kind",
    "--file": "file",
    "--model": "model",
    "--commit-hash": "commit_hash",
    "--id": "id",
    "--occurred-at": "occurred_at"
  };
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--ai" || argument === "--background") {
      event[argument.slice(2)] = true;
      continue;
    }
    if (argument === "--additions" || argument === "--deletions" || argument === "--exit-code") {
      const parsed = integer(args[index + 1]);
      if (parsed === null) {
        process.exitCode = 2;
        return;
      }
      event[argument.slice(2).replace("-", "_")] = parsed;
      index += 1;
      continue;
    }
    if (!values[argument] || typeof args[index + 1] !== "string") {
      process.exitCode = 2;
      return;
    }
    event[values[argument]] = args[index + 1];
    index += 1;
  }
  if (!event.project_path || !kinds.has(event.kind)) {
    process.exitCode = 2;
    return;
  }
  await sendEvent(event);
}

main().catch(() => {
  process.exitCode = 2;
});
