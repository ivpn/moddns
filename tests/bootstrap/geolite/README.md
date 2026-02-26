These are stub .mmdb files for integration tests, NOT full GeoLite2 databases.

They contain only two entries (AS15169 Google, AS13335 Cloudflare).
City lookups return empty records but won't crash.

Regenerate: `cd tests && python3 scripts/generate_stub_mmdb.py`
