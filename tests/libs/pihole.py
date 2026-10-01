"""Drive the Pi-hole container of the E2E stack.

Pi-hole forwards plain DNS only. The documented way to put it in front of modDNS
is a local dnscrypt-proxy consuming the profile's DoH stamp; this helper starts
that dnscrypt-proxy *inside* the Pi-hole container (the address Pi-hole already
forwards to, see docker-compose.yml) and swaps the stamp per test.

This is the second of two ways the suite runs dnscrypt-proxy. libs/dnscrypt_proxy.py
runs it as a *host* subprocess (test_dnscrypt_proxy.py) and consumes the API's
stamp unmodified, because the stamp encodes the host loopback. Here the same
pinned static binary (from ``libs.dnscrypt_proxy.resolve_binary``) is copied into
the Pi-hole container and started there, so the stamp must first be re-addressed
to the proxy's compose address (libs/stamps.py); the profile in the DoH path is
untouched.
"""
from __future__ import annotations

import io
import tarfile
import socket
import time
from typing import Callable

import docker
from dns import message, query

from libs.dnscrypt_proxy import render_toml

_DIR = "/opt/dnscrypt-proxy"
_PORT = 5353  # FTLCONF_dns_upstreams in docker-compose.yml
_CA_IN_CONTAINER = "/certs/dev-ca.crt"


class Pihole:
    def __init__(self, dns_addr: str, dnscrypt_binary: str, container_name: str = "pihole"):
        host, _, port = dns_addr.rpartition(":")
        # dnspython dials an address, not a name; resolve once via the C library.
        self.dns_host, self.dns_port = socket.gethostbyname(host), int(port)
        self._binary = dnscrypt_binary
        self._container = docker.from_env().containers.get(container_name)

    def wait_ready(self, timeout: float = 120.0) -> None:
        """FTL answers ``pi.hole`` itself, so an answer means Pi-hole is serving."""
        deadline = time.monotonic() + timeout
        while True:
            try:
                if self.query("pi.hole", timeout=2.0).answer:
                    return
            except Exception:  # noqa: BLE001 — not up yet
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError(f"Pi-hole at {self.dns_host}:{self.dns_port} did not come up within {timeout}s")
            time.sleep(1.0)

    def use_stamp(self, stamp: str) -> None:
        """(Re)start the in-container dnscrypt-proxy on ``stamp``.

        The binary and its config travel in via ``put_archive`` (``docker cp``),
        which works regardless of where the host keeps the binary; the process is
        started detached with the dev CA as its TLS trust for the proxy's cert."""
        self._container.exec_run(["pkill", "-x", "dnscrypt-proxy"])
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            tf.add(self._binary, arcname="dnscrypt-proxy")
            toml = render_toml(_PORT, stamp).encode()
            ti = tarfile.TarInfo("dnscrypt-proxy.toml")
            ti.size = len(toml)
            tf.addfile(ti, io.BytesIO(toml))
        self._container.exec_run(["mkdir", "-p", _DIR])
        self._container.put_archive(_DIR, buf.getvalue())
        self._container.exec_run(
            ["sh", "-c", f"cd {_DIR} && exec ./dnscrypt-proxy -config dnscrypt-proxy.toml > dnscrypt-proxy.log 2>&1"],
            environment={"SSL_CERT_FILE": _CA_IN_CONTAINER},
            detach=True,
        )

    def dnscrypt_log(self) -> str:
        _, out = self._container.exec_run(["cat", f"{_DIR}/dnscrypt-proxy.log"])
        return out.decode(errors="replace")

    def query(self, domain: str, rdtype: str = "A", timeout: float = 5.0) -> message.Message:
        q = message.make_query(domain, rdtype)
        return query.udp(q, self.dns_host, port=self.dns_port, timeout=timeout)

    def wait_until(
        self, domain: str, predicate: Callable[[message.Message], bool],
        rdtype: str = "A", timeout: float = 30.0, interval: float = 0.5,
    ) -> message.Message:
        """Poll through Pi-hole until ``predicate`` holds (dnscrypt-proxy may still be starting)."""
        deadline = time.monotonic() + timeout
        while True:
            resp = None
            try:
                resp = self.query(domain, rdtype)
                if predicate(resp):
                    return resp
            except Exception:  # noqa: BLE001 — SERVFAIL/timeout while the forwarder starts
                pass
            if time.monotonic() >= deadline:
                if resp is not None:
                    return resp
                raise TimeoutError(f"no answer from Pi-hole for {domain} within {timeout}s\n{self.dnscrypt_log()}")
            time.sleep(interval)
