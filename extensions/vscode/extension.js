"use strict";

const vscode = require("vscode");
const fs = require("fs");
const http = require("http");
const os = require("os");
const path = require("path");
const crypto = require("crypto");

const AGENT_PORT = 47653;
const HEARTBEAT_MS = 30_000;
let disposed = false;
let lastSignalAt = 0;
let pending = [];
let sending = false;
let configCache = { at: 0, value: null };

function configPath() {
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

async function localConfig() {
  if (Date.now() - configCache.at < 2_000) {
    return configCache.value;
  }
  try {
    const parsed = JSON.parse(await fs.promises.readFile(configPath(), "utf8"));
    if (!parsed || typeof parsed.local_token !== "string" || !parsed.local_token) {
      throw new Error("local token is missing");
    }
    configCache = { at: Date.now(), value: parsed };
  } catch (_) {
    configCache = { at: Date.now(), value: null };
  }
  return configCache.value;
}

function postJSON(route, payload) {
  return new Promise(async (resolve) => {
    const config = await localConfig();
    if (!config || disposed) {
      resolve(false);
      return;
    }
    let body;
    try {
      body = JSON.stringify(payload);
    } catch (_) {
      resolve(false);
      return;
    }
    const request = http.request({
      hostname: "127.0.0.1",
      port: AGENT_PORT,
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
      response.on("end", () => resolve(response.statusCode >= 200 && response.statusCode < 300));
    });
    request.setTimeout(1_000, () => request.destroy());
    request.on("error", () => resolve(false));
    request.end(body);
  });
}

async function drainEvents() {
  if (sending) {
    return;
  }
  sending = true;
  while (pending.length && !disposed) {
    const event = pending.shift();
    // Do not retry or grow a backlog while the local agent is unavailable.
    if (!(await postJSON("/v1/events", event))) {
      pending = [];
      break;
    }
  }
  sending = false;
}

function emit(projectPath, kind, file, extra) {
  if (disposed || !projectPath) {
    return;
  }
  lastSignalAt = Date.now();
  const event = Object.assign({
    id: crypto.randomBytes(16).toString("hex"),
    occurred_at: new Date().toISOString(),
    project_path: projectPath,
    source: "vscode",
    kind,
    file: file || "",
    additions: 0,
    deletions: 0,
    ai: false,
    background: false
  }, extra || {});
  if (pending.length >= 200) {
    pending.shift();
  }
  pending.push(event);
  void drainEvents();
}

function documentMetadata(document) {
  if (!document || document.uri.scheme !== "file" || document.isClosed) {
    return null;
  }
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  if (!folder || folder.uri.scheme !== "file") {
    return null;
  }
  const relative = path.relative(folder.uri.fsPath, document.uri.fsPath);
  if (!relative || relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
    return null;
  }
  return { projectPath: folder.uri.fsPath, file: relative.split(path.sep).join("/") };
}

function focusedDocument(document) {
  if (!vscode.window.state.focused || !vscode.window.activeTextEditor) {
    return false;
  }
  return vscode.window.activeTextEditor.document.uri.toString() === document.uri.toString();
}

function emitDocument(document, kind, force) {
  if (!force && !focusedDocument(document)) {
    return;
  }
  const metadata = documentMetadata(document);
  if (metadata) {
    emit(metadata.projectPath, kind, metadata.file);
  }
}

function focusedEditor() {
  if (vscode.window.state.focused && vscode.window.activeTextEditor) {
    emitDocument(vscode.window.activeTextEditor.document, "focus", true);
  }
}

function taskProject(task) {
  const scope = task.scope;
  if (scope && scope.uri && scope.uri.scheme === "file") {
    return scope.uri.fsPath;
  }
  if (scope === vscode.TaskScope.Workspace && vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length === 1) {
    return vscode.workspace.workspaceFolders[0].uri.fsPath;
  }
  return null;
}

function taskKind(task) {
  // Names are inspected only to classify the event and are never transmitted.
  const label = `${task.definition && task.definition.type || ""} ${task.name || ""}`.toLowerCase();
  return label.includes("test") ? "test" : "build";
}

function activate(context) {
  disposed = false;
  const tasks = new WeakMap();
  const debugSessions = new WeakMap();

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      if (editor && vscode.window.state.focused) {
        emitDocument(editor.document, "focus", true);
      }
    }),
    vscode.window.onDidChangeWindowState((state) => {
      if (state.focused) {
        focusedEditor();
      }
    }),
    vscode.workspace.onDidChangeTextDocument((change) => {
      // Deliberately do not inspect change.contentChanges or document text.
      emitDocument(change.document, "edit", false);
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      emitDocument(document, "save", false);
    }),
    vscode.tasks.onDidStartTask((event) => {
      if (!vscode.window.state.focused) {
        return;
      }
      const projectPath = taskProject(event.execution.task);
      if (!projectPath) {
        return;
      }
      const metadata = { projectPath, kind: taskKind(event.execution.task) };
      tasks.set(event.execution, metadata);
      emit(projectPath, metadata.kind, "");
    }),
    vscode.tasks.onDidEndTaskProcess((event) => {
      const metadata = tasks.get(event.execution);
      if (!metadata) {
        return;
      }
      tasks.delete(event.execution);
      const extra = typeof event.exitCode === "number" ? { exit_code: event.exitCode } : {};
      emit(metadata.projectPath, metadata.kind, "", extra);
    }),
    vscode.debug.onDidStartDebugSession((session) => {
      if (!vscode.window.state.focused || !session.workspaceFolder || session.workspaceFolder.uri.scheme !== "file") {
        return;
      }
      const projectPath = session.workspaceFolder.uri.fsPath;
      debugSessions.set(session, projectPath);
      emit(projectPath, "debug", "");
    }),
    vscode.debug.onDidTerminateDebugSession((session) => {
      const projectPath = debugSessions.get(session);
      if (!projectPath) {
        return;
      }
      debugSessions.delete(session);
      emit(projectPath, "debug", "");
    }),
    vscode.commands.registerCommand("gritloom.pause", async () => {
      const duration = await vscode.window.showInputBox({ prompt: "Pause GritLoom for (for example: 30m or 1h)", value: "1h" });
      if (!duration) {
        return;
      }
      const ok = await postJSON("/v1/pause", { duration });
      if (ok) {
        vscode.window.showInformationMessage(`GritLoom tracking paused for ${duration}.`);
      } else {
        vscode.window.showWarningMessage("GritLoom local agent is unavailable or rejected the pause request.");
      }
    }),
    vscode.commands.registerCommand("gritloom.resume", async () => {
      const ok = await postJSON("/v1/resume", {});
      if (ok) {
        vscode.window.showInformationMessage("GritLoom tracking resumed.");
      } else {
        vscode.window.showWarningMessage("GritLoom local agent is unavailable or rejected the resume request.");
      }
    })
  );

  const heartbeat = setInterval(() => {
    if (Date.now() - lastSignalAt >= HEARTBEAT_MS) {
      focusedEditor();
    }
  }, 5_000);
  context.subscriptions.push({ dispose: () => clearInterval(heartbeat) });
}

function deactivate() {
  disposed = true;
  pending = [];
}

module.exports = { activate, deactivate };
