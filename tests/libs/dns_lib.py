import asyncio
import glob
import os
import time
from typing import Callable, Optional

import httpx
from dns import resolver, message
from dns.query import https as query_https, tls as query_tls, quic as query_quic
from dns.message import Message, ShortHeader

# Sentinel answers the proxy returns for blocked domains.
BLOCKED_IPV4 = "0.0.0.0"
BLOCKED_IPV6 = "::"
BLOCKED_IPS = (BLOCKED_IPV4, BLOCKED_IPV6)

# Where the proxy binds inside the docker-compose host network. Stamps encode
# a publicly-routable anycast IP (cfg.Server.ServerAddresses[0]); for live
# integration we substitute the loopback bind while preserving SNI so the
# proxy's profile-id dispatcher still resolves the right tenant.
LOCAL_PROXY_HOST = "127.0.0.1"


def first_answer_ip(resp: Message) -> Optional[str]:
    """First IP string from the answer section, or None if there is no answer."""
    if not resp.answer:
        return None
    return resp.answer[0].to_text().split(" ")[-1]


def is_blocked(resp: Message) -> bool:
    """The answer is one of the proxy's block sentinels."""
    return first_answer_ip(resp) in BLOCKED_IPS


def is_resolved(resp: Message) -> bool:
    """There is an answer and it is not a block sentinel."""
    ip = first_answer_ip(resp)
    return ip is not None and ip not in BLOCKED_IPS


def answer_ip_is(expected: str) -> Callable[[Message], bool]:
    """Predicate factory: the first answer IP equals ``expected``."""
    return lambda resp: first_answer_ip(resp) == expected


def assert_blocked(resp: Message, domain: str = "domain") -> None:
    """Assert the response is the proxy's block sentinel (0.0.0.0 / ::)."""
    assert resp.answer, f"Expected a blocked answer for {domain}, got empty answer"
    ip = first_answer_ip(resp)
    assert ip in BLOCKED_IPS, f"{domain} was not blocked; got {ip}"


def assert_not_blocked(resp: Message, domain: str = "domain") -> None:
    """Assert the response is NOT the proxy's block sentinel.

    An empty answer (NXDOMAIN/NODATA) or a CNAME-first answer counts as "not
    blocked" — blocking always yields a synthetic 0.0.0.0/:: answer, so only
    the sentinel itself is a failure. When the test also requires the domain to
    genuinely resolve, poll with ``wait_until(..., is_resolved)`` first.
    """
    if not resp.answer:
        return
    ip = first_answer_ip(resp)
    assert ip not in BLOCKED_IPS, f"{domain} was unexpectedly blocked (got {ip})"


def _mkcert_ca_path() -> str:
    """Locate the mkcert dev CA bundle. Override via MODDNS_TEST_CA_PATH."""
    override = os.getenv("MODDNS_TEST_CA_PATH")
    if override:
        return override
    matches = sorted(glob.glob("/home/maciek/git/dns/certs/mkcert_development_CA_*.crt"))
    if not matches:
        raise RuntimeError(
            "mkcert dev CA not found at certs/mkcert_development_CA_*.crt — "
            "set MODDNS_TEST_CA_PATH or regenerate the test certs"
        )
    return matches[0]


class DNSLib:
    def __init__(self, server: str):
        self.server = server
        self.my_resolver = resolver.Resolver(configure=False)
        self.my_resolver.nameservers = [self.server]

    async def send_doh_request(self, profile_id: str, domain: str, record_type: str) -> Message:
        with httpx.Client() as client:
            query = message.make_query(domain, record_type)
            r = query_https(
                query,
                f"{self.server}{profile_id}",
                session=client,
                resolver=self.my_resolver,
            )
            return r

    async def send_doh_request_with_retry(
        self, profile_id: str, domain: str, record_type: str,
        retries: int = 5, delay: float = 3.0,
    ) -> Message:
        """Retry DoH requests to tolerate transient proxy unavailability (e.g. during Redis failover recovery)."""
        last_err = None
        for attempt in range(retries):
            try:
                return await self.send_doh_request(profile_id, domain, record_type)
            except (ShortHeader, httpx.ConnectError, httpx.ReadError, OSError) as e:
                last_err = e
                if attempt < retries - 1:
                    await asyncio.sleep(delay)
        raise last_err

    async def wait_until(
        self, profile_id: str, domain: str, record_type: str,
        predicate: Callable[[Message], bool],
        *, timeout: float = 10.0, interval: float = 0.25,
    ) -> Message:
        """Poll a DoH query until ``predicate(resp)`` is truthy or ``timeout`` expires.

        Returns the last response either way — callers keep their normal
        assertions after the wait, so a timeout surfaces as the usual assertion
        failure carrying the real (stale) answer.

        Why this exists: the API writes profile settings to the Redis master
        while the proxy reads the replica, so a profile/rule/blocklist mutation
        is not visible to DNS resolution until replication catches up. Route the
        first query after any mutation through this helper.

        Only poll for POSITIVE conditions. A negative assertion ("must NOT be
        blocked") polled this way passes instantly on a stale read that predates
        the mutation ever applying — instead, first wait for a companion
        positive effect of the same mutation to propagate, then assert the
        negative with a plain query.
        """
        deadline = time.monotonic() + timeout
        while True:
            try:
                resp = await self.send_doh_request(profile_id, domain, record_type)
            except (ShortHeader, httpx.ConnectError, httpx.ReadError, OSError):
                # The proxy drops connections for unknown profiles, so a freshly
                # created profile can cause ShortHeader until it propagates to
                # the replica. Treat as "not ready yet"; re-raise on deadline.
                if time.monotonic() >= deadline:
                    raise
                await asyncio.sleep(interval)
                continue
            try:
                if predicate(resp):
                    return resp
            except Exception:
                pass  # e.g. malformed/partial answer while state is still stale
            if time.monotonic() >= deadline:
                return resp
            await asyncio.sleep(interval)

    async def send_via_stamp(self, stamp, domain: str, record_type: str) -> Message:
        """Dispatch a DNS query through the protocol encoded in a parsed dnsstamps stamp.

        Connects to LOCAL_PROXY_HOST (loopback) but uses the stamp's hostname for SNI
        — that's what carries profile-id dispatch through the proxy. The mkcert dev
        CA is used to verify TLS; the cert SANs include *.ivpndns.com so per-profile
        subdomains validate.
        """
        from dnsstamps import Protocol  # local import — only needed when this helper is used

        query = message.make_query(domain, record_type)
        ca = _mkcert_ca_path()

        if stamp.protocol == Protocol.DOH:
            url = f"https://{stamp.hostname}{stamp.path}"
            with httpx.Client(verify=ca) as client:
                return query_https(query, url, session=client)
        if stamp.protocol == Protocol.DOT:
            port = _port_from_address(stamp.address, default=853)
            return query_tls(
                query, LOCAL_PROXY_HOST, port=port,
                server_hostname=stamp.hostname, verify=ca,
            )
        if stamp.protocol == Protocol.DOQ:
            port = _port_from_address(stamp.address, default=853)
            return query_quic(
                query, LOCAL_PROXY_HOST, port=port,
                server_hostname=stamp.hostname, verify=ca,
            )
        raise ValueError(f"unsupported stamp protocol: {stamp.protocol}")


def _port_from_address(address: str, default: int) -> int:
    """Extract :PORT suffix from a stamp's address field. Falls back to default."""
    if ":" in address:
        try:
            return int(address.rsplit(":", 1)[1])
        except ValueError:
            pass
    return default
