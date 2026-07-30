"""Run the real `dnscrypt-proxy` client against the local stack in E2E tests.

DNSCrypt support in modDNS is currently delivered as per-profile DoH stamps consumed by
the `dnscrypt-proxy` client (see docs/features/dnscrypt/). This module provisions
the official static `dnscrypt-proxy` binary and drives it as a host subprocess so
tests can prove a real client resolves through modDNS with the profile carried in
the DoH URL path.

There is no official dnscrypt-proxy Docker image; the static binary is pinned by
version + sha256 (cleaner provenance than a community image) and consumes the API
DoH stamp unmodified (the local stack encodes 127.0.0.1 + moddns.dev into it).

The binary is resolved via, in order:
  1. MODDNS_DNSCRYPT_PROXY_BIN env var (explicit path — CI sets this).
  2. A checksum-verified download of the pinned release, cached under the temp dir.
Non-linux/x86_64 hosts without the env override skip the test.
"""

from __future__ import annotations

import hashlib
import os
import platform
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Optional

from dns import message, query

# Pinned release. SHA256 is of the official linux_x86_64 tarball asset:
#   https://github.com/DNSCrypt/dnscrypt-proxy/releases/download/2.1.18/dnscrypt-proxy-linux_x86_64-2.1.18.tar.gz
PINNED_VERSION = "2.1.18"
_ASSET = "dnscrypt-proxy-linux_x86_64-{v}.tar.gz"
_ASSET_URL = "https://github.com/DNSCrypt/dnscrypt-proxy/releases/download/{v}/" + _ASSET
_ASSET_SHA256 = "c8c8acb35b0f6619bfe8e4eed0c192672f8fd1964f467a42881905814e261c3e"

_CACHE_DIR = Path(tempfile.gettempdir()) / "moddns-dnscrypt-proxy" / PINNED_VERSION


def _sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


class _UnsupportedPlatform(RuntimeError):
    """Raised when auto-download can't serve this platform (no env override)."""


def ensure_binary() -> str:
    """Return a path to the pinned `dnscrypt-proxy` binary, downloading + verifying
    it if needed. **Raises** on any failure — use this from CI / scripts (and the
    module's ``__main__``). Tests should use :func:`resolve_binary`, which converts
    unavailability into a pytest skip.

    Honors MODDNS_DNSCRYPT_PROXY_BIN; otherwise downloads the pinned, checksum-
    verified release for linux/x86_64. Mirrors the env-first spirit of
    ``libs.dns_lib._dev_ca_path``.
    """
    override = os.getenv("MODDNS_DNSCRYPT_PROXY_BIN")
    if override:
        p = Path(override).resolve()
        if not p.is_file():
            raise RuntimeError(f"MODDNS_DNSCRYPT_PROXY_BIN={override} but file does not exist")
        return str(p)

    if platform.system() != "Linux" or platform.machine() not in ("x86_64", "amd64"):
        raise _UnsupportedPlatform(
            "dnscrypt-proxy auto-download supports linux/x86_64 only; "
            "set MODDNS_DNSCRYPT_PROXY_BIN to run elsewhere"
        )

    cached = _CACHE_DIR / "dnscrypt-proxy"
    if cached.is_file() and os.access(cached, os.X_OK):
        return str(cached)

    url = _ASSET_URL.format(v=PINNED_VERSION)
    _CACHE_DIR.mkdir(parents=True, exist_ok=True)
    tarball = _CACHE_DIR / _ASSET.format(v=PINNED_VERSION)
    with urllib.request.urlopen(url, timeout=60) as resp, tarball.open("wb") as out:
        out.write(resp.read())

    digest = _sha256(tarball)
    if digest != _ASSET_SHA256:
        raise RuntimeError(
            f"dnscrypt-proxy tarball sha256 mismatch: got {digest}, want {_ASSET_SHA256}"
        )

    with tarfile.open(tarball) as tf:
        member = next((m for m in tf.getmembers() if m.name.endswith("/dnscrypt-proxy") or m.name == "dnscrypt-proxy"), None)
        if member is None:
            raise RuntimeError("dnscrypt-proxy binary not found inside the release tarball")
        member.name = "dnscrypt-proxy"  # flatten
        tf.extract(member, path=_CACHE_DIR)
    cached.chmod(0o755)
    return str(cached)


def resolve_binary() -> str:
    """Test-facing resolver: like :func:`ensure_binary` but converts an unavailable
    binary (unsupported platform, or an offline/network download failure) into a
    ``pytest.skip`` so local runs without connectivity don't hard-fail. A checksum
    mismatch still raises — that's tampering/corruption, not unavailability."""
    import pytest  # local import: keeps ensure_binary()/__main__ pytest-free

    try:
        return ensure_binary()
    except _UnsupportedPlatform as exc:
        pytest.skip(str(exc))
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        pytest.skip(f"could not download dnscrypt-proxy {PINNED_VERSION}: {exc}")


def _free_udp_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


_TOML = """\
listen_addresses = ['127.0.0.1:{port}']
server_names = ['modDNS-test']
ipv6_servers = false
require_dnssec = false
require_nolog = false
require_nofilter = false
cache = false
bootstrap_resolvers = ['9.9.9.9:53']
netprobe_timeout = 0

[static]
  [static.'modDNS-test']
  stamp = '{stamp}'
"""


class DnscryptProxyClient:
    """A `dnscrypt-proxy` subprocess bound to a single modDNS DoH stamp.

    Use as a context manager. ``query`` sends a plain-UDP DNS query to the local
    listener and returns a ``dns.message.Message`` (feed it to the block-sentinel
    helpers in ``libs.dns_lib``).
    """

    def __init__(self, stamp: str, ca_path: str, binary: str, port: Optional[int] = None):
        self._stamp = stamp
        self._ca_path = ca_path
        self._binary = binary
        self.port = port or _free_udp_port()
        self._proc: Optional[subprocess.Popen] = None
        self._workdir: Optional[tempfile.TemporaryDirectory] = None
        self._logpath: Optional[Path] = None

    def _read_log(self) -> str:
        if self._logpath and self._logpath.is_file():
            return self._logpath.read_text(errors="replace")
        return ""

    def start(self, expect_ready: bool = True, timeout: float = 20.0) -> "DnscryptProxyClient":
        self._workdir = tempfile.TemporaryDirectory(prefix="dcp-")
        wd = Path(self._workdir.name)
        cfg = wd / "dnscrypt-proxy.toml"
        cfg.write_text(_TOML.format(port=self.port, stamp=self._stamp))
        self._logpath = wd / "dnscrypt-proxy.log"

        env = dict(os.environ)
        # dnscrypt-proxy is a Go binary; Go's TLS honors SSL_CERT_FILE for the
        # DoH connection, so it trusts the dev CA the local proxy is signed with.
        env["SSL_CERT_FILE"] = self._ca_path

        with self._logpath.open("wb") as log:
            self._proc = subprocess.Popen(
                [self._binary, "-config", str(cfg)],
                stdout=log, stderr=subprocess.STDOUT, env=env,
            )

        # "Now listening" means the socket is bound; "OK (DoH)" means the resolver
        # answered dnscrypt-proxy's test query (only happens for a valid profile).
        deadline = time.monotonic() + timeout
        listening = False
        while time.monotonic() < deadline:
            if self._proc.poll() is not None and expect_ready:
                raise RuntimeError(
                    f"dnscrypt-proxy exited early (code {self._proc.returncode}):\n{self._read_log()}"
                )
            log = self._read_log()
            listening = "Now listening" in log
            if expect_ready and "OK (DoH)" in log and listening:
                return self
            if not expect_ready and listening:
                # Give the bogus resolver a moment to be marked unusable, then proceed.
                time.sleep(1.0)
                return self
            time.sleep(0.2)

        if expect_ready:
            raise RuntimeError(
                f"dnscrypt-proxy did not become ready within {timeout}s:\n{self._read_log()}"
            )
        return self  # expect_ready=False: proceed even if never fully up

    def query(self, domain: str, rdtype: str = "A", timeout: float = 5.0) -> message.Message:
        q = message.make_query(domain, rdtype)
        return query.udp(q, "127.0.0.1", port=self.port, timeout=timeout)

    def stop(self) -> None:
        if self._proc and self._proc.poll() is None:
            self._proc.terminate()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()
                self._proc.wait(timeout=5)
        if self._workdir:
            self._workdir.cleanup()

    def __enter__(self) -> "DnscryptProxyClient":
        return self

    def __exit__(self, *exc) -> None:
        self.stop()


if __name__ == "__main__":
    # CI entrypoint: resolve (download + checksum-verify) the pinned binary and
    # print ONLY its path to stdout, so the workflow can export it as
    # MODDNS_DNSCRYPT_PROXY_BIN. Version + URL + sha256 live here (single source
    # of truth); any failure raises → non-zero exit → the CI step fails loudly.
    print(ensure_binary())
