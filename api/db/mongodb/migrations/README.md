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

### Query logs collections

Note: Query logs time-series collections are created by the proxy service. Their only index is the `{profile_id, timestamp}` meta+time index MongoDB creates automatically on time-series creation (≥6.3) — no code creates query-log indexes explicitly (verified against prod, moddns-shadow#688).
