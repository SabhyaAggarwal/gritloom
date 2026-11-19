# Reviewer manifest format

Every reviewer link exposes two representations of the same immutable submission:

- `GET /api/v1/submissions/{token}` returns the reviewer-view JSON, including `manifest_hash`.
- `GET /api/v1/submissions/{token}/manifest` returns the exact versioned manifest bytes covered by that hash.

The token contains 256 bits of randomness before encoding and is stored only as a server-side hash. Anyone who possesses the link can read the selected evidence. Tokens cannot be listed or recovered from submission history.

## Verify the bytes

```sh
curl -fsS https://gritloom.example/api/v1/submissions/REVIEW_TOKEN/manifest -o manifest.json
curl -fsS https://gritloom.example/api/v1/submissions/REVIEW_TOKEN -o submission.json
sha256sum manifest.json
```

On macOS, use `shasum -a 256 manifest.json`. Compare the lowercase hexadecimal digest with `manifest_hash` in `submission.json`. The raw endpoint also returns the same value in `X-Gritloom-Manifest-SHA256` and checks the stored hash before serving the document.

Do not parse and re-serialize the raw manifest before hashing: whitespace, field order, timestamp formatting, and HTML escaping change the digest. The endpoint deliberately returns the canonical bytes without an added newline.

## Version 1

The manifest is a JSON object with fields in this order:

| Field | Meaning |
| --- | --- |
| `version` | Integer `1`. |
| `submission_id` | Random server identifier for this submission. |
| `author` | Username at creation time. |
| `created_at` | UTC RFC 3339 timestamp, truncated to milliseconds at creation. |
| `note` | Builder-provided context, up to 2,000 bytes. |
| `sessions` | Selected session snapshots, sorted oldest first with session ID as the tie-breaker. |

Each session contains its ID, project ID/name/color, start/end timestamps, exact seconds, kind, sources, verification label, iteration counts, evidence hash, and granular evidence array. Only `engaged` sessions are accepted for a submission. Background AI events can appear as labeled evidence inside an engaged session, but standalone `background_ai` sessions cannot be submitted as engaged time.

Evidence events contain timing, source, kind, optional repository-relative file, addition/deletion counts, optional process exit code, AI/background markers, optional model name, optional commit hash, and optional branch. They never intentionally contain source text, edit contents, prompt text, tool input/output, terminal output, screenshots, keystrokes, transcripts, or absolute local paths.

## Interpretation limits

A matching digest establishes snapshot integrity after creation. It does not establish that the local client was honest, that two different clients cannot fabricate equivalent metadata, or that the work meets a program’s definition of productive struggle. Reviewers should inspect the selected evidence alongside the shipped project and builder context.

Profile/project visibility is independent of existing submissions. Making a project private hides it from the public profile but does not rewrite or revoke a previously shared snapshot.
