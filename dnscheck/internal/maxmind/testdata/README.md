Stub MaxMind databases for dnscheck unit tests, NOT full GeoLite2 files.

- `GeoLite2-ASN.mmdb` — database type `GeoLite2-ASN`, a handful of entries
  (8.8.8.8 → AS15169 GOOGLE, Cloudflare / Apple / Microsoft ranges). Any other
  IP yields an empty record with no error.
- `GeoLite2-City.mmdb` — database type `GeoLite2-City`. Used as the
  wrong-database-type case for the startup guard; never queried for content.

Regenerate from the repo's `tests/` directory (needs its venv):

    cd tests && source venv/bin/activate && \
      python scripts/generate_stub_mmdb.py --out-dir ../dnscheck/internal/maxmind/testdata --city-typed
