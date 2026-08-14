### Important

golang-migrate for mongoDB uses `db.runCommand( { <command> } )` syntax: https://www.mongodb.com/docs/manual/reference/command/

https://pkg.go.dev/github.com/golang-migrate/migrate/v4/database/mongodb#section-readme


### Query logs collections

Note: Query logs time-series collections are created by the proxy service. Their only index is the `{profile_id, timestamp}` meta+time index MongoDB creates automatically on time-series creation (≥6.3) — no code creates query-log indexes explicitly (verified against prod, moddns-shadow#688).
