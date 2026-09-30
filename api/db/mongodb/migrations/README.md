### Important

golang-migrate for mongoDB uses `db.runCommand( { <command> } )` syntax: https://www.mongodb.com/docs/manual/reference/command/

https://pkg.go.dev/github.com/golang-migrate/migrate/v4/database/mongodb#section-readme


### Migration 025 (accounts email lowercase)

Backfills `accounts.email` to lowercase (`$toLower`; idempotent via `$expr` filter).
The unique `email` index (migration 013) makes this fail mid-update — dirty migration,
API startup blocked — if two accounts differ only in casing. Before deploying, audit
the target environment: case-collision groups must be zero, and emails must be ASCII
without surrounding whitespace (`$toLower` is ASCII-only; the migration does not trim).
Audit queries are in the PR that introduced the migration.

### Migration 026 (statistics clear)

Clears the legacy per-profile `statistics` time-series collection. Query statistics are
now service-wide and live in `service_statistics` (a regular collection the proxy upserts
into, one document per PoP and hour, no TTL);
per-profile statistics return with the Analytics page in a new shape. `delete` with an empty
filter, not `drop`, so a fresh database without the collection succeeds; an empty-filter
delete on a time-series collection needs MongoDB ≥ 7.0. Idempotent; the down migration is
a no-op. Deploy note: proxies still running the previous release between the DCN and DFN
restarts may recreate `statistics` as a plain collection; after the DFN restart, drop it if
`db.statistics.countDocuments({})` is non-zero.

### Migration 027 (statistics retention collections)

Creates the three per-profile statistics time-series collections the proxy writes to:
`statistics_30d`, `statistics_90d`, `statistics_1y` (`timeField` `bucket_start`, `metaField`
`meta` = `{profile_id, device_id}`, granularity `minutes`, `expireAfterSeconds` 2592000 /
7776000 / 31536000) and a `{meta.profile_id: 1, bucket_start: 1}` index on each (the automatic
`{meta, bucket_start}` index cannot serve a `meta.profile_id` predicate; explain-verified on 7.0.8
and 8.2.3 for the statistics read and both purge deletes), then drops the legacy `statistics` collection (already emptied by 026).
The proxy never creates these collections, so deploy the migration before the proxy release.
Verified on mongo:7.0.8 with golang-migrate: `drop` of a missing namespace returns `ok: 1`, so
the migration also applies to a fresh database. `create` is not idempotent (an existing
collection fails with `NamespaceExists`), so if a run is interrupted after some `create`
commands, drop the partially created `statistics_*` collections and `force 26` before retrying.
The index build carries no `commitQuorum` because standalone MongoDB (dev, E2E) rejects it; on
the degraded production replica set, build with `commitQuorum: "majority"` or fix a hung build
with `setIndexCommitQuorum` (see the note on 018-style hangs). The down migration drops the three
collections and their indexes (data is lost) and does not recreate the legacy `statistics`
collection; after it, an older proxy's `InsertMany` would auto-create plain (non-time-series,
no TTL) collections under the same names, so drop those before re-running `up`. Proxies still on the previous release may recreate `statistics`
as a plain collection after the drop; drop it again once every PoP runs the new release.

### Query logs collections

Note: Query logs time-series collections are created by the proxy service. Their only index is the `{profile_id, timestamp}` meta+time index MongoDB creates automatically on time-series creation (≥6.3) — no code creates query-log indexes explicitly (verified against prod, moddns-shadow#688).
