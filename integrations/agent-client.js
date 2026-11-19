"use strict";

const fs = require("fs");
const http = require("http");
const os = require("os");
const path = require("path");
const crypto = require("crypto");

function agentConfigPath() {
  let base;
  if (process.platform === "win32") {
    base = process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming");
  } else if (process.platform === "darwin") {
    base = path.join(os.homedir(), "Library", "Application Support");
  } else {
    base = process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config");
  }
  return path.join(base, "gritloom", "agent.json");
}

function randomEventID() {
  return crypto.randomBytes(16).toString("hex");
}

function safeModel(value) {
  return typeof value === "string" && value.length <= 100 && !/[\x00-\x1f\x7f]/.test(value) ? value : "";
}

// Metadata-only allowlist. Unknown fields, including prompt, tool_input,
// source_contents, content, command, and transcript paths, are never forwarded.
function eventPayload(event) {
  const payload = {
    id: typeof event.id === "string" ? event.id : randomEventID(),
    occurred_at: event.occurred_at || new Date().toISOString(),
    project_path: event.project_path,
    source: event.source || "generic",
    kind: event.kind,
    file: event.file || "",
    additions: Number.isInteger(event.additions) ? event.additions : 0,
    deletions: Number.isInteger(event.deletions) ? event.deletions : 0,
    ai: event.ai === true,
    background: event.background === true,
    model: safeModel(event.model),
    commit_hash: typeof event.commit_hash === "string" ? event.commit_hash : ""
  };
  if (Number.isInteger(event.exit_code)) {
    payload.exit_code = event.exit_code;
  }
  return payload;
}

// Resolve rather than reject: hook and editor processes should remain quiet
// and successful when the local agent is offline or paused.
function postLocal(route, payload, timeoutMs = 1_000) {
  return new Promise((resolve) => {
    let config;
    let body;
    try {
      config = JSON.parse(fs.readFileSync(agentConfigPath(), "utf8"));
      if (!config || typeof config.local_token !== "string" || !config.local_token) {
        resolve(false);
        return;
      }
      body = JSON.stringify(payload);
    } catch (_) {
      resolve(false);
      return;
    }
    let settled = false;
    const finish = (value) => {
      if (!settled) {
        settled = true;
        resolve(value);
      }
    };
    const request = http.request({
      hostname: "127.0.0.1",
      port: 47653,
      family: 4,
      path: route,
      method: "POST",
      headers: {
        Authorization: `Bearer ${config.local_token}`,
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(body)
      },
      agent: false
    }, (response) => {
      response.resume();
      response.on("end", () => finish(response.statusCode >= 200 && response.statusCode < 300));
    });
    request.setTimeout(timeoutMs, () => request.destroy());
    request.on("error", () => finish(false));
    request.end(body);
  });
}

function sendEvent(event) {
  if (!event || typeof event.project_path !== "string" || !event.project_path || typeof event.kind !== "string") {
    return Promise.resolve(false);
  }
  return postLocal("/v1/events", eventPayload(event));
}

// Claude hook payloads can contain prompt/tool content. Input is capped and
// discarded after callers extract their small metadata allowlist; it is never
// logged, persisted, or sent onward.
function readHookJSON(maximumBytes = 256 * 1024) {
  return new Promise((resolve) => {
    let size = 0;
    let tooLarge = false;
    const chunks = [];
    process.stdin.on("data", (chunk) => {
      size += chunk.length;
      if (size > maximumBytes) {
        tooLarge = true;
        chunks.length = 0;
      } else if (!tooLarge) {
        chunks.push(chunk);
      }
    });
    process.stdin.on("end", () => {
      if (tooLarge) {
        resolve(null);
        return;
      }
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString("utf8")));
      } catch (_) {
        resolve(null);
      }
    });
    process.stdin.on("error", () => resolve(null));
  });
}

module.exports = { agentConfigPath, eventPayload, postLocal, randomEventID, readHookJSON, sendEvent };
