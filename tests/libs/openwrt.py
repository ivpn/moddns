"""Drive the OpenWrt router containers of the E2E stack.

The commands come from the app module behind Setup > Routers > OpenWrt
(``app/src/components/setup/routerCommands.ts``), rendered with node and run
verbatim in the router's shell, both package-manager branches included, as a
customer pastes them. The stack stands in for the internet: plain DNS from the
router to addresses outside the compose network is answered by knot
(``tests/openwrt/entrypoint.sh``). That stand-in would also answer a bootstrap
pointed at modDNS's public addresses, so the unit test RouterCommands.test.ts is
what keeps ``bootstrap_dns`` out of the guide.
"""
from __future__ import annotations

import json
import shutil
import socket
import subprocess
import time
from collections.abc import Callable
from pathlib import Path

import docker
from dns import message, query

_ROUTER_COMMANDS = Path(__file__).resolve().parents[2] / "app/src/components/setup/routerCommands.ts"


def render_openwrt_commands(doh_endpoint: str) -> str:
    """The OpenWrt command block exactly as the Setup page renders it."""
    node = shutil.which("node")
    if node is None:
        raise RuntimeError("node is required to render the app's router commands")
    script = (
        f"import({json.dumps(_ROUTER_COMMANDS.as_uri())})"
        f".then(m => process.stdout.write(m.buildOpenWrtCommands({json.dumps({'dohEndpoint': doh_endpoint})})))"
    )
    out = subprocess.run([node, "--no-warnings", "-e", script], capture_output=True, text=True)
    if out.returncode != 0:
        # Type stripping needs Node 22.18 or later.
        version = subprocess.run([node, "--version"], capture_output=True, text=True).stdout.strip()
        raise RuntimeError(f"node {version} could not render {_ROUTER_COMMANDS.name}:\n{out.stderr}")
    return out.stdout


class OpenWrtRouter:
    def __init__(self, container_name: str, dns_addr: str):
        host, _, port = dns_addr.rpartition(":")
        # dnspython dials an address, not a name; resolve once via the C library.
        self.dns_host, self.dns_port = socket.gethostbyname(host), int(port)
        self.name = container_name
        self._container = docker.from_env().containers.get(container_name)

    def wait_ready(self, timeout: float = 120.0) -> None:
        """dnsmasq answers ``localhost`` from /etc/hosts once the router is serving."""
        deadline = time.monotonic() + timeout
        while True:
            try:
                if self.query("localhost", timeout=2.0).answer:
                    return
            except Exception:
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError(f"{self.name} did not serve DNS within {timeout}s\n{self.log()}")
            time.sleep(1.0)

    def run_guide(self, commands: str) -> str:
        """Run the pasted block in the router shell; returns the combined output.

        Not ``set -e``: on each release one package-manager branch is missing,
        exactly as when a customer pastes the whole block."""
        _, out = self._container.exec_run(["sh", "-c", commands])
        return out.decode(errors="replace")

    def uci_get(self, key: str) -> str:
        code, out = self._container.exec_run(["uci", "-q", "get", key])
        return out.decode(errors="replace").strip() if code == 0 else ""

    def log(self) -> str:
        _, out = self._container.exec_run(["sh", "-c", "logread | grep -iE 'https-dns-proxy|dnsmasq' | grep -v cgroup | tail -20"])
        return out.decode(errors="replace")

    def query(self, domain: str, rdtype: str = "A", timeout: float = 5.0) -> message.Message:
        q = message.make_query(domain, rdtype)
        return query.udp(q, self.dns_host, port=self.dns_port, timeout=timeout)

    def wait_until(
        self, domain: str, predicate: Callable[[message.Message], bool],
        rdtype: str = "A", timeout: float = 30.0, interval: float = 0.5,
    ) -> message.Message:
        """Poll through the router until ``predicate`` holds (https-dns-proxy may still be starting)."""
        deadline = time.monotonic() + timeout
        while True:
            resp = None
            try:
                resp = self.query(domain, rdtype)
                if predicate(resp):
                    return resp
            except Exception:
                pass
            if time.monotonic() >= deadline:
                if resp is not None:
                    return resp
                raise TimeoutError(f"no answer from {self.name} for {domain} within {timeout}s\n{self.log()}")
            time.sleep(interval)
