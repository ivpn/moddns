This is a stub GeoLite2-ASN .mmdb for backend E2E tests, NOT the full GeoLite2 database.

It contains only the networks the tests query (Google, Cloudflare, Apple, Microsoft ASNs).
The proxy and dnscheck read the ASN edition only. `GeoLite2-Country.mmdb` is a second stub (US for the same networks) read by the API only, for the top-clients enrichment.

Regenerate: `cd tests && python3 scripts/generate_stub_mmdb.py --country`
