# Security and privacy guide

No software can honestly guarantee zero vulnerabilities or zero misuse. GritLoom uses small trust boundaries and automated checks, but it has not had an independent security audit. Review this document and test your deployment before using it for consequential decisions.

## Server controls

- Passwords are hashed with bcrypt at cost 12 and limited to 12–128 UTF-8 bytes.
- Session, device, and reviewer tokens are random 256-bit values, base64url encoded. Only SHA-256 token hashes are stored. Device tokens are shown once and can be revoked without deleting history.
- Session cookies are `HttpOnly`, `SameSite=Lax`, scoped to `/`, and expire after 30 days. Production uses `Secure` and the `__Host-` prefix.
- State-changing browser routes enforce same-origin requests. JSON decoding rejects unknown fields, trailing documents, invalid content types, and oversized bodies.
- Security headers include a restrictive Content Security Policy, `X-Content-Type-Options`, `Referrer-Policy: no-referrer`, frame denial, and a restrictive Permissions Policy. Production also enables HSTS.
- Device ingestion accepts 1–500 events per batch, up to 2 MiB, with per-device rate and daily event quotas. Events have strict IDs, kinds, sources, timestamp bounds, relative-path validation, and project-hash validation.
- Registration and login have in-memory IP/account rate limits. Failed logins perform a dummy bcrypt comparison.
- SQL uses bound parameters. User-generated text is rendered through React rather than injected HTML.
- The single process has read-header/read/write-idle timeouts and bounded request headers. Put additional connection, request-rate, and account-abuse controls at the production proxy.

Rate limits are per process and reset on restart. They are not a substitute for a production WAF, registration policy, abuse monitoring, or a persistent distributed limiter.

## Local-agent boundary

The agent listens only on `127.0.0.1:47653`. It requires a random local bearer token and rejects browser origins, cross-site fetch metadata, and non-loopback Host headers. Remote server URLs must use HTTPS; plain HTTP is allowed only for loopback development.

Configuration and queue files are written with mode `0600`, using atomic replacement. Exclusion paths remain local. The queue is acknowledged only after the server accepts or confirms every event in the batch. Pausing stops new collection, not upload of events already queued.

Integrations use metadata allowlists. They do not forward prompts, tool payloads, source contents, terminal output, transcript paths, or edit contents. A malicious or compromised integration running as the user can still read the local token and submit false metadata, just as it could act as that user in other local tools.

## Privacy decisions

Profiles and projects are public by default to match the bounty model. Users can make the entire profile private or toggle each project independently. Private-project sessions, totals, daily activity, and AI intervals are excluded from public responses, not merely hidden in the browser.

Reviewer links are capability URLs. They reveal only the selected snapshots, but anyone with the URL can read them. Visibility changes do not alter immutable reviewer snapshots. The UI warns about this at project controls and submission creation.

The server stores granular event metadata for reconstruction even though the ordinary dashboard omits it. Only selected snapshots expose granular evidence to a reviewer. Protect the SQLite volume and backups accordingly.

## Production checklist

- Use HTTPS with secure cookies; do not expose the development configuration publicly.
- Set `GRITLOOM_TRUST_PROXY=true` only when requests arrive through a trusted proxy that overwrites forwarding headers.
- Preserve the original Host header so same-origin validation continues to work.
- Keep the agent API loopback-only; never forward or expose port 47653.
- Run the container as the supplied non-root user, keep capabilities dropped, and restrict database/volume access.
- Add proxy-level body, connection, and request-rate limits, especially for public profiles and account creation.
- Back up SQLite consistently and test restoration. Treat backups as sensitive data.
- Monitor disk growth, event rejection rates, ingestion latency, and account abuse.
- Keep Go, Node, npm packages, and the base image patched; run tests, race checks, dependency audits, and browser checks after upgrades.
- Do not claim productive-struggle approval from `review_ready` or a valid manifest hash.
