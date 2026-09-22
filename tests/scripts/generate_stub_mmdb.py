"""Generate minimal GeoLite2-ASN stub .mmdb for CI/test use.

Contains only the IPs used in e2e tests:
  - 8.8.8.8/32      -> AS15169 (Google)
  - 104.16.0.0/13   -> AS13335 (Cloudflare, covers 104.16.x-104.23.x)
  - 104.24.0.0/14   -> AS13335 (Cloudflare, covers 104.24.x-104.27.x)
  - 17.0.0.0/8      -> AS714   (Apple, covers all 17.x.x.x)
  - 13.104.0.0/14   -> AS8075  (Microsoft, covers 13.104.x-13.107.x)
  - 150.171.0.0/16  -> AS8075  (Microsoft, covers 150.171.x.x)

The Cloudflare ranges cover IPs returned by cloudflare.com A records
(e.g. 104.18.74.230) and HTTPS ipv4hint values (e.g. 104.16.132.229).

Apple owns the entire 17.0.0.0/8 block (AS714).

Microsoft's microsoft.com resolves to IPs in 13.104.0.0/14 (AS8075).
The 150.171.0.0/16 range is also announced by AS8075.

Usage:
    python scripts/generate_stub_mmdb.py
        Writes the backend E2E stubs to bootstrap/geolite/ (both files carry
        the ASN payload; the "City" file is a copy so mounts never fail).

    python scripts/generate_stub_mmdb.py --out-dir ../dnscheck/internal/maxmind/testdata --city-typed
        Writes the dnscheck unit-test fixtures. --city-typed makes the City
        file a real GeoLite2-City database so a wrong-type file can be tested.
"""

import argparse
import os

from netaddr import IPSet
from mmdb_writer import MMDBWriter

parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
parser.add_argument("--out-dir", default="bootstrap/geolite", help="directory to write the .mmdb files into")
parser.add_argument(
    "--city-typed",
    action="store_true",
    help="write GeoLite2-City.mmdb with database_type GeoLite2-City instead of copying the ASN stub",
)
args = parser.parse_args()
os.makedirs(args.out_dir, exist_ok=True)

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

writer.insert_network(
    IPSet(["17.0.0.0/8"]),
    {"autonomous_system_number": 714, "autonomous_system_organization": "APPLE-ENGINEERING"},
)

writer.insert_network(
    IPSet(["13.104.0.0/14", "150.171.0.0/16"]),
    {"autonomous_system_number": 8075, "autonomous_system_organization": "MICROSOFT-CORP-MSN-AS-BLOCK"},
)

out_asn = os.path.join(args.out_dir, "GeoLite2-ASN.mmdb")
writer.to_db_file(out_asn)
print(f"Wrote {out_asn}")

out_city = os.path.join(args.out_dir, "GeoLite2-City.mmdb")
if args.city_typed:
    city_writer = MMDBWriter(
        ip_version=4,
        database_type="GeoLite2-City",
        description={"en": "Stub GeoLite2-City for unit tests"},
    )
    city_writer.insert_network(
        IPSet(["8.8.8.8/32"]),
        {"country": {"iso_code": "US", "names": {"en": "United States"}}},
    )
    city_writer.to_db_file(out_city)
else:
    writer.to_db_file(out_city)
print(f"Wrote {out_city}")
