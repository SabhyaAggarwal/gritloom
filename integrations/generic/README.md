# Generic metadata plugin and git commits

All integrations use `../agent-client.js`, which reads the local bearer token from the platform GritLoom config and sends only to `127.0.0.1:47653`. It has no npm dependencies and fails quietly when the agent is offline.

## Command-line example

```sh
node /absolute/path/to/gritloom/integrations/generic/send-event.js \
  --project "$PWD" --kind save --file src/example.js

node /absolute/path/to/gritloom/integrations/generic/send-event.js \
  --project "$PWD" --kind test --exit-code 0
```

The accepted event kinds are `focus`, `edit`, `save`, `ai_start`, `ai_end`, `ai_tool`, `build`, `test`, `debug`, and `commit`. Other optional metadata flags are `--additions`, `--deletions`, `--exit-code`, `--ai`, `--background`, `--model`, `--commit-hash`, `--id`, and `--occurred-at`. Do not put prompt or source text in any argument; the client allowlist has no field for either.

A minimal custom plugin is only metadata:

```js
const { sendEvent } = require("/absolute/path/to/gritloom/integrations/agent-client");

function onEditorFocus(workspacePath) {
  void sendEvent({
    project_path: workspacePath, // raw path goes only to the loopback agent
    source: "generic",
    kind: "focus",
    file: "src/example.js"      // repository-relative; never document contents
  });
}
```

The agent converts `project_path` to exactly one lowercase 64-character SHA-256 identifier and infers the project name, branch, and sanitized repository URL before persistent queuing or upload.

## Git post-commit hook

Symlink `git-post-commit.js` as the executable `post-commit` file in the repository's hooks directory, or make that hook a small script which runs `node /absolute/path/to/gritloom/integrations/generic/git-post-commit.js`. One way to locate the hooks directory is:

```sh
git rev-parse --git-path hooks
```

A direct copy is not sufficient because the adapter loads `../agent-client.js` relative to its installed location. The script runs only `git rev-parse --show-toplevel HEAD` and sends the commit hash plus local project path to the agent. It does not run `git show`, read diffs, or collect source contents. Set `GRITLOOM_AI=1` only when the commit itself is AI activity; set `GRITLOOM_BACKGROUND=1` only for unattended AI work.
