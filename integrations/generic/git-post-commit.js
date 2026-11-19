#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const { sendEvent } = require("../agent-client");

async function main() {
  let output;
  try {
    output = execFileSync("git", ["rev-parse", "--show-toplevel", "HEAD"], {
      cwd: process.cwd(),
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      timeout: 2_000,
      maxBuffer: 16 * 1024
    });
  } catch (_) {
    return;
  }
  const lines = output.trim().split(/\r?\n/);
  if (lines.length < 2 || !/^[a-fA-F0-9]{7,64}$/.test(lines[lines.length - 1])) {
    return;
  }
  const commitHash = lines.pop();
  const projectPath = lines.join("\n");
  await sendEvent({
    project_path: projectPath,
    source: "generic",
    kind: "commit",
    commit_hash: commitHash,
    ai: process.env.GRITLOOM_AI === "1",
    background: process.env.GRITLOOM_BACKGROUND === "1"
  });
}

main().catch(() => {}).finally(() => {
  process.exitCode = 0;
});
