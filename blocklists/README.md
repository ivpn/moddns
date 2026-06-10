# DNS Blocklists

Cron-driven service that maintains the platform's public DNS blocklists. It
downloads third-party and curated domain lists, extracts and validates the
domains, and publishes them to a shared Redis set that the **proxy reads on the
hot path of every DNS query** (`SISMEMBER blocklist:<id> <fqdn>`). A copy of the
content and per-list metadata is also stored in MongoDB for the API to serve.

## How it works

```
sources/**.json  ──ReadSources──►  per-source cron schedule (@hourly)
                                          │
                                          ▼  ProcessBlocklist (per list)
   download (HTTP GET, 30s, TLS 1.2+, ≤100MB)
        │  reject if response exceeds the size limit (truncation guard)
        ▼
   ExtractMetadata     ── Last-Modified / Version / entry count from headers
        │
        ▼
   Convert             ── format-specific extraction to one domain per line
        │
        ▼
   normalize + validate (shared gate, internal/extractor):
        strip BOM/CR/whitespace/trailing-dot, lowercase; drop comments,
        wildcards and non-domain junk; punycode-aware (IDN xn-- TLDs kept)
        │
        ▼
   validation gate     ── abort the swap if 0 domains, or a shrink beyond
        │                  BLOCKLIST_SHRINK_THRESHOLD vs the previous run
        ▼
   publish (the SAME validated set to both stores):
     • Redis  ── temp set built via SADD (5000/chunk), then atomic RENAME swap
     • Mongo  ── content chunked at 100k domains/doc + metadata upsert
```

On a rejected/failed update, the previously published Redis and Mongo data are
left untouched.

## Sources

The directory of source definitions is given by `UPDATER_SOURCES_DIR` (mounted at
`/sources` in the container). Each file is a JSON array of source objects,
grouped by purpose:

- `<sources>/3rd_party/` — AdGuard, Hagezi, OISD, Steven Black
- `<sources>/categories/` — gambling, adult, social_media, etc.
- `<sources>/security/` — threat-intelligence feeds

> **Local development vs production.** The `blocklists/sources/` directory in
> this repo is for **local development only**. The blocklists deployed to
> production are maintained in the separate `dns-deployment` repository and
> mounted into the container as `/sources`. Keep the two in sync when adding or
> changing a source; `TestConfiguredSourcesAreRoutable` guards that every source
> in `blocklists/sources/` routes to a known extractor.

The **extractor is selected by the `blocklist_id` prefix**
(`internal/extractor/extractor.go`):

| Prefix | Extractor | Format |
|---|---|---|
| `adguard*` | AdGuard | AdBlock rules (`\|\|domain^`, `!` comments, modifiers) |
| `hagezi*` | Hagezi | plain domains, `#` headers (strict metadata) |
| `oisd*` | OISD | plain domains (`domainswild2`), `#` headers |
| `steven_black*` | Steven Black | hosts file (`0.0.0.0 domain`) |
| `blp_*`, `ut1_*`, `shadowwhisperer_*`, `someonewhocares_*`, `peter_lowe*`, `1hosts_*` | Domains | plain or hosts-format domains, graceful metadata (optional `Last modified`/`Last updated`/`Count` headers), hosts-tolerant |

Key source fields: `blocklist_id` (required, routes the extractor), `name`,
`description`, `source_url`, `kind` (`general`/`category`),
`category` (e.g. `gambling`, `security`), `intensity`, `default`,
`schedule` (cron). See any file under
`sources/` for a complete example.

The authoritative behaviour spec is
[`docs/specs/blocklists-processing-behaviour.md`](../docs/specs/blocklists-processing-behaviour.md)
(decision tables for download, extraction, validation, caching, scheduling).

## Observability

The service exposes a small HTTP server (default port **9153**, set
`METRICS_PORT=0` to disable):

| Endpoint | Purpose |
|---|---|
| `GET /metrics` | Prometheus metrics |
| `GET /health/live` | liveness — always 200 while the process is up |
| `GET /health/ready` | readiness — 200 only if MongoDB **and** Redis are reachable (503 otherwise) |

### Metrics

All metrics are labelled by `source` (the `blocklist_id`).

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `blocklists_update_total` | Counter | `source`, `status` (`success`/`failure`) | Update attempts by outcome |
| `blocklists_update_duration_seconds` | Histogram | `source` | End-to-end update duration |
| `blocklists_domains_extracted` | Gauge | `source` | Validated domains published in the last update |
| `blocklists_source_declared_entries` | Gauge | `source` | Entry count the source reports — header value, or the service's own non-comment line count when no header is present (a divergence signal against `domains_extracted`) |
| `blocklists_last_success_timestamp_seconds` | Gauge | `source` | Unix time of the last successful update |
| `blocklists_download_bytes` | Gauge | `source` | Bytes downloaded in the last update |
| `blocklists_validation_rejected_total` | Counter | `source`, `reason` (`empty`/`shrink`/`scan_error`/`truncated`) | Swaps rejected by the validation gate |

**Recommended alerts:**

- *Stale list* — `time() - blocklists_last_success_timestamp_seconds` exceeds a
  threshold (e.g. a few hours). This is the primary signal for a wedged updater,
  important because the service is a singleton writer.
- *Repeated rejections* — `rate(blocklists_validation_rejected_total[1h]) > 0`,
  especially `reason="empty"`/`"shrink"`/`"truncated"`, indicates a bad or
  changed upstream.
- *Declared-vs-saved divergence* —
  `blocklists_domains_extracted / blocklists_source_declared_entries` well below
  1 means many source entries were dropped (malformed lines or a partial
  download). A ratio slightly under 1 is normal (dedup + dropped invalids), so
  alert on large/sudden drops, not exact equality.
- *Update failures* — `rate(blocklists_update_total{status="failure"}[1h])`.

Errors and panics are also reported to **Sentry** (Warn level and above) when a
DSN is configured.

## Configuration

All configuration is via environment variables.

**Database (MongoDB)**

| Var | Default | Notes |
|---|---|---|
| `DB_URI` | — | connection string |
| `DB_NAME` | — | database name |
| `DB_USERNAME` / `DB_PASSWORD` | — | credentials |
| `DB_AUTH_SOURCE` | `dns` | auth database |
| `DB_MIGRATIONS_SOURCE` | — | migrations source |

**Cache (Redis, Sentinel-aware)**

| Var | Default | Notes |
|---|---|---|
| `CACHE_ADDRESS` | — | single-node address |
| `CACHE_ADDRESSES` | — | comma-separated Sentinel addresses |
| `CACHE_MASTER_NAME` | — | Sentinel master name |
| `CACHE_USERNAME` / `CACHE_PASSWORD` | — | Redis ACL credentials |
| `CACHE_FAILOVER_USERNAME` / `CACHE_FAILOVER_PASSWORD` | — | Sentinel credentials |
| `CACHE_TLS_ENABLED` | `false` | enable TLS |
| `CACHE_CERT_FILE` / `CACHE_KEY_FILE` / `CACHE_CA_CERT_FILE` | — | TLS material |
| `CACHE_TLS_INSECURE_SKIP_VERIFY` | `false` | **never `true` in production** |

**Updater**

| Var | Default | Notes |
|---|---|---|
| `UPDATER_TYPE` | `standard` | updater implementation |
| `UPDATER_SOURCES_DIR` | — | directory of source JSON files |
| `BLOCKLIST_SHRINK_THRESHOLD` | `0.5` | max fraction a list may shrink before the swap is rejected (clamped to `[0,1]`) |

**Metrics / observability**

| Var | Default | Notes |
|---|---|---|
| `METRICS_PORT` | `9153` | `/metrics` + `/health/*` port; `0` disables the server |
| `SENTRY_DSN` / `SENTRY_ENVIRONMENT` / `SENTRY_RELEASE` | — | Sentry reporting |
| `SERVER_NAME` | — | instance name for logs |

## Development

```bash
go build ./...
go test ./...            # full suite (offline)
go test -race ./...

# Real-world parsing fixtures (network): download + sample real upstream lists
REFRESH_FIXTURES=1 go test ./service -run TestRefreshFixtures -count=1
```

`service/testdata/real/` holds frozen, truncated snapshots of every distinct
source syntax; `TestRealBlocklists` runs them through the parse pipeline offline.
See `service/testdata/real/README.md` for the source/license list.
