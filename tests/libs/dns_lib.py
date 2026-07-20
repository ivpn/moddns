import asyncio
import time
from typing import Callable, Optional

import httpx
from dns import resolver, message
from dns.query import https as query_https
from dns.message import Message, ShortHeader

# Sentinel answers the proxy returns for blocked domains.
BLOCKED_IPV4 = "0.0.0.0"
BLOCKED_IPV6 = "::"
BLOCKED_IPS = (BLOCKED_IPV4, BLOCKED_IPV6)


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
            resp = await self.send_doh_request(profile_id, domain, record_type)
            try:
                if predicate(resp):
                    return resp
            except Exception:
                pass  # e.g. malformed/partial answer while state is still stale
            if time.monotonic() >= deadline:
                return resp
            await asyncio.sleep(interval)
