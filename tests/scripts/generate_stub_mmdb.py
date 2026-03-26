"""Generate minimal GeoLite2-ASN stub .mmdb for CI/test use.

Contains only the IPs used in e2e tests:
  - 8.8.8.8/32      -> AS15169 (Google)
  - 104.16.0.0/13   -> AS13335 (Cloudflare, covers 104.16.x-104.23.x)
  - 104.24.0.0/14   -> AS13335 (Cloudflare, covers 104.24.x-104.27.x)

The Cloudflare ranges cover IPs returned by cloudflare.com A records
(e.g. 104.18.74.230) and HTTPS ipv4hint values (e.g. 104.16.132.229).

Usage:
    python scripts/generate_stub_mmdb.py
"""

from netaddr import IPSet
from mmdb_writer import MMDBWriter

writer = MMDBWriter(
    ip_version=4,
    database_type="GeoLite2-ASN",
    description={"en": "Stub GeoLite2-ASN for CI tests"},
)

writer.insert_network(
    IPSet(["8.8.8.8/32"]),
    {"autonomous_system_number": 15169, "autonomous_system_organization": "GOOGLE"},
)

writer.insert_network(
    IPSet(["104.16.0.0/13", "104.24.0.0/14"]),
    {
        "autonomous_system_number": 13335,
        "autonomous_system_organization": "CLOUDFLARENET",
    },
)

out_asn = "bootstrap/geolite/GeoLite2-ASN.mmdb"
writer.to_db_file(out_asn)
print(f"Wrote {out_asn}")

out_city = "bootstrap/geolite/GeoLite2-City.mmdb"
writer.to_db_file(out_city)
print(f"Wrote {out_city}")
