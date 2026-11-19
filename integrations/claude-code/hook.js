#!/usr/bin/env node
"use strict";

const { sendEvent } = require("../agent-client");

const kinds = {
  "user-prompt": "ai_start",
  "pre-tool": "ai_tool",
  "post-tool": "ai_tool",
  "stop": "ai_end"
};

async function main() {
  const action = process.argv[2];
  const kind = kinds[action];
  if (!kind) {
    return;
  }
  const { readHookJSON } = require("../agent-client");
  const hook = await readHookJSON();
  if (!hook || typeof hook.cwd !== "string" || !hook.cwd) {
    return;
  }
  // Only cwd and model are read from the hook object. prompt, tool_input,
  // tool_response, command, transcript_path, and all content fields are left
  // untouched and disappear with this short-lived process.
  await sendEvent({
    project_path: hook.cwd,
    source: "claude-code",
    kind,
    ai: true,
    background: process.argv.includes("--background") || process.env.GRITLOOM_BACKGROUND === "1",
    model: hook.model
  });
}

main().catch(() => {}).finally(() => {
  process.exitCode = 0;
});
