# GritLoom Local Activity for VS Code

A dependency-free extension for VS Code, Cursor, Windsurf, and VSCodium. It reports editor focus, focused edits and saves, task process outcomes, debug transitions, and a focused heartbeat no more often than every 30 seconds to the local GritLoom agent.

## Setup

1. Run `gritloom-agent setup --server URL --token DEVICE_TOKEN`, then `gritloom-agent run`.
2. Install this directory as an extension, or package it as a VSIX and install it with your editor's `--install-extension` command.

The extension reads `local_token` from `gritloom/agent.json` in the platform user config directory (normally `~/.config/gritloom/agent.json` on Linux). Only `127.0.0.1:47653` is contacted. If the agent is stopped or paused, events are dropped quietly; the extension does not show connection-error notifications.

The **GritLoom: Pause Tracking** and **GritLoom: Resume Tracking** commands use the local authenticated API.

## Privacy boundary

The extension never reads or transmits document text, edit contents, prompt text, keystrokes, terminal output, debug output, or task commands. It transmits only event timing, a random event ID, the workspace path to the loopback agent, a repository-relative file path, and task/debug outcome metadata. The agent replaces the local workspace path with a SHA-256 project identifier before storage in its upload queue or transmission to a server.
