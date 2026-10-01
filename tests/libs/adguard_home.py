"""Drive the AdGuard Home container of the E2E stack through its /control API.

AdGuard Home resolves the hostnames of its encrypted upstreams through its own
bootstrap resolvers, never the OS resolver, so every call takes the bootstrap
list explicitly — that is the knob a misconfigured deployment gets wrong.
"""
from __future__ import annotations

import socket
import time
from typing import Optional

import requests
from dns import message, query

_HTTP_TIMEOUT = 10.0
# AdGuard Home probes every upstream in the request; a dead bootstrap costs the
# full upstream timeout (10s) before it reports the failure.
_TEST_TIMEOUT = 90.0


class AdGuardHome:
    def __init__(self, api_addr: str, dns_addr: str, bootstrap: list[str]):
        self.api = api_addr.rstrip("/")
        host, _, port = dns_addr.rpartition(":")
        # dnspython dials an address, not a name; resolve once via the C library.
        self.dns_host, self.dns_port = socket.gethostbyname(host), int(port)
        self.bootstrap = list(bootstrap)

    def wait_ready(self, timeout: float = 60.0) -> None:
        deadline = time.monotonic() + timeout
        while True:
            try:
                r = requests.get(f"{self.api}/control/status", timeout=_HTTP_TIMEOUT)
                if r.ok and r.json().get("running"):
                    return
            except requests.RequestException:
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError(f"AdGuard Home at {self.api} did not come up within {timeout}s")
            time.sleep(0.5)

    def set_upstreams(self, upstreams: list[str], bootstrap: Optional[list[str]] = None) -> None:
        """Point AdGuard Home at ``upstreams`` (any form it accepts: URL or sdns://)."""
        r = requests.post(
            f"{self.api}/control/dns_config",
            json={"upstream_dns": upstreams, "bootstrap_dns": bootstrap or self.bootstrap},
            timeout=_HTTP_TIMEOUT,
        )
        r.raise_for_status()

    def test_upstreams(self, upstreams: list[str], bootstrap: Optional[list[str]] = None) -> dict[str, str]:
        """The web UI's *Test upstreams* button: each upstream maps to ``"OK"`` or an error."""
        r = requests.post(
            f"{self.api}/control/test_upstream_dns",
            json={
                "upstream_dns": upstreams,
                "bootstrap_dns": bootstrap or self.bootstrap,
                "private_upstream": [],
                "fallback_dns": [],
            },
            timeout=_TEST_TIMEOUT,
        )
        r.raise_for_status()
        return r.json()

    def query(self, domain: str, rdtype: str = "A", timeout: float = 15.0) -> message.Message:
        q = message.make_query(domain, rdtype)
        return query.udp(q, self.dns_host, port=self.dns_port, timeout=timeout)
