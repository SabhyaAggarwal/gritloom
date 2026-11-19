# Deploy GritLoom

The intended small deployment is one unprivileged container, one persistent volume, and a TLS reverse proxy on the same host. No managed database or external asset service is required.

## 1. Build and run locally

```sh
docker compose up --build
```

Compose listens on `127.0.0.1:8080` and uses development cookies so local HTTP sign-in works. Data persists in the `gritloom-data` named volume. The container runs as a non-root user, has a read-only root filesystem with a bounded writable `/tmp`, drops all capabilities, and enables `no-new-privileges`.

## 2. Configure production

Use a real hostname and HTTPS. Start Compose with production settings:

```sh
GRITLOOM_ENV=production \
GRITLOOM_SECURE_COOKIES=true \
GRITLOOM_TRUST_PROXY=true \
docker compose up -d --build
```

Keep the supplied loopback port binding when the proxy runs on the same host. Do not expose port 8080 directly to the internet.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port inside the container/process. |
| `GRITLOOM_ENV` | development outside image; production in image | Selects production cookie/HSTS defaults. |
| `GRITLOOM_SECURE_COOKIES` | `true` in production | Requires HTTPS cookies and enables HSTS. |
| `GRITLOOM_TRUST_PROXY` | `false` | Trusts the first valid `X-Forwarded-For` address for rate limiting. Enable only behind a trusted proxy that overwrites inbound forwarding headers. |
| `GRITLOOM_DATA_DIR` | `/data` in image | Directory containing `gritloom.db`. |
| `GRITLOOM_WEB_ROOT` | `/app/web/dist` in image | Built static assets served by Go. |

The application has no required static API secret. Session, device, local-agent, and reviewer tokens are generated randomly.

## 3. TLS proxy example

This nginx location preserves the host used by same-origin checks and supports server-sent events:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_read_timeout 75s;
}
```

Configure the surrounding `server` block with your hostname and valid certificates, and add proxy-level request/body/connection rate limits. Do not forward the local agent port 47653.

If clients or another untrusted proxy can reach the application directly, leave `GRITLOOM_TRUST_PROXY=false`; otherwise clients could spoof the address used by login and registration limits.

## 4. Health and updates

```sh
curl -fsS http://127.0.0.1:8080/api/v1/health
docker compose logs --tail=100 gritloom
```

Before an update, back up the database, then rebuild:

```sh
docker compose up -d --build
```

Schema creation is automatic and additive in this MVP. Review migrations before upgrading a populated production instance.

## 5. Backup and restore

The database contains granular metadata, account hashes, and token hashes. Protect backups as sensitive user data.

For a simple cold backup, stop the application, copy the named volume with your established volume-backup tool, and restart it. Do not copy only the main database file while WAL writes are active; a live copy can omit committed data in `-wal` or be inconsistent. Test restoration in a separate directory before relying on the backup.

For larger deployments, use SQLite’s online backup mechanism through an operational tool that is explicitly installed for that purpose, and retain both the backup policy and restoration test results.

## 6. Resource and cost limits

Start with one right-sized VPS and an attached SSD volume. Measure CPU, memory, disk growth/IOPS, network, backup duration, SSE connections, and p95/p99 ingestion latency under realistic load. Add alerts before capacity becomes critical.

The architecture avoids recurring per-user API and managed-database costs, but no target-scale load test or sub-$500/month guarantee has been established. In particular, full-user timeline reconstruction on ingestion must be benchmarked and made incremental before accepting the bounty’s full stated scale. See [the architecture notes](../docs/architecture.md).

Run one SQLite-backed application instance. Horizontal replicas need a deliberate shared event fan-out and a database/filesystem design that preserves SQLite’s locking semantics.
