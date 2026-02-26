"""Generate minimal GeoLite2-ASN stub .mmdb for CI/test use.

Contains only the IPs used in e2e tests:
  - 8.8.8.8/32       -> AS15169 (Google)
  - 104.18.74.230/32 -> AS13335 (Cloudflare)

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
    IPSet(["104.18.74.230/32"]),
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
