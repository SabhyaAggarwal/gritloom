# Contributing to GritLoom

Keep the tool useful for reflection first and review second. Do not introduce engagement scores, attention claims, prompt/source collection, screenshots, keylogging, or opaque anti-fraud judgments.

## Development

Use Go 1.26.11+ and Node.js 18+. Run the quick-start commands in the README, then the full checks before proposing a change:

```sh
gofmt -w path/to/changed.go
go test -count=1 ./...
go test -race -count=1 ./internal/tracking ./internal/agent ./internal/server
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
npm --prefix web run build
npm --prefix web audit
```

Run `node --check` on changed integration JavaScript. For visual changes, render the affected desktop and mobile states and inspect the result. Exercise real registration/device/project/submission behavior when changing those flows.

## Invariants

- Engaged and background AI time remain separate.
- Overlapping sources never double-count time.
- Unknown project-switch and idle gaps are not invented.
- Raw paths never enter the upload queue or server database.
- Private-project data never appears in public responses.
- Reviewer snapshots remain immutable and include only selected sessions.
- A valid hash is evidence of snapshot integrity, not proof of effort or truth.
- The agent API stays loopback-only and bearer authenticated; remote agent uploads require HTTPS.

Add a focused regression test for behavior fixes. Preserve offline retry/idempotency and run race tests when changing concurrency. Keep new dependencies to a minimum, especially in the local agent.

By contributing, you agree that your contribution is licensed under AGPLv3.
