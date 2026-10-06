"""Helpers shared by the statistics E2E tests (consented writes, read endpoints).

Statistics buckets are 15 minutes wide and flushed when closed or when the
proxy shuts down (proxy-statistics-behaviour Y18), so tests force the flush by
restarting the proxy container.
"""

import asyncio
import re
from datetime import datetime

import docker
from dns import message
from dns.query import tls as query_tls
from dns.rdatatype import A

from libs.constants import RESOLVABLE_TEST_DOMAIN
from libs.dns_lib import LOCAL_PROXY_HOST, _dev_ca_path, is_resolved

PROXY_CONTAINER = "dnsproxy"
# Replica lag between the API's settings write and the proxy's read.
SETTINGS_SETTLE_S = 2
REASON_KEYS = {"blocklist", "service", "custom_rule", "rebinding", "default_rule", "other"}
PROTOCOL_KEYS = {"doh", "dot", "doq"}
# Time-series tiers (api-endpoint-behaviour J11): fixed 15-minute and 1-hour
# collections plus three day-tier collections routed by retention.
TIER_15MIN = "statistics_15min"
TIER_1H = "statistics_1h"
DAY_TIERS = ("statistics_1d_30d", "statistics_1d_90d", "statistics_1d_1y")
STATS_COLLECTIONS = (TIER_15MIN, TIER_1H, *DAY_TIERS)
# Stored documents are flat (proxy-statistics-behaviour Y19).
COUNTER_KEYS = (
    {"total", "blocked", "dnssec"}
    | {f"reason_{k}" for k in REASON_KEYS}
    | {f"proto_{k}" for k in PROTOCOL_KEYS}
)


def restart_container():
    d = docker.from_env()
    try:
        d.containers.get(PROXY_CONTAINER).restart(timeout=30)
    finally:
        d.close()


async def restart_proxy(user):
    """Graceful restart: the collector flushes open buckets on shutdown (Y18)."""
    await asyncio.to_thread(restart_container)
    # A profile that never consents, so the probe leaves nothing to flush later.
    resp = await user.wait_for(
        user.new_profile("ready"), RESOLVABLE_TEST_DOMAIN, A, is_resolved, timeout=60.0, interval=1.0
    )
    assert is_resolved(resp), "proxy did not come back after restart"


def patch_stats(user, pid, value):
    user.patch_setting(pid, "/settings/statistics/enabled", value)


def dot_query(pid, device, domain):
    """DoT SNI carries the device as {device}-{profile}.<server name> (clientid.go)."""
    q = message.make_query(domain, A)
    return query_tls(
        q, LOCAL_PROXY_HOST, port=853,
        server_hostname=f"{device}-{pid}.moddns.dev", verify=_dev_ca_path(),
    )


async def enable_and_warm(user, pid):
    """Warm the profile with statistics still off so warm-up queries are never counted."""
    resp = await user.wait_for(pid, RESOLVABLE_TEST_DOMAIN, A, is_resolved)
    assert is_resolved(resp)
    patch_stats(user, pid, True)
    await asyncio.sleep(SETTINGS_SETTLE_S)


def parse_ts(value: str) -> datetime:
    """Parse an API timestamp; Python 3.10 ``fromisoformat`` rejects 1-2 fractional digits."""
    value = value.replace("Z", "+00:00")
    return datetime.fromisoformat(re.sub(r"\.(\d+)", lambda m: "." + m.group(1).ljust(6, "0")[:6], value))
