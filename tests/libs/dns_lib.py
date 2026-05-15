import glob
import os
import time

import httpx
from dns import resolver, message
from dns.query import https as query_https, tls as query_tls, quic as query_quic
from dns.message import Message, ShortHeader

# Where the proxy binds inside the docker-compose host network. Stamps encode
# a publicly-routable anycast IP (cfg.Server.ServerAddresses[0]); for live
# integration we substitute the loopback bind while preserving SNI so the
# proxy's profile-id dispatcher still resolves the right tenant.
LOCAL_PROXY_HOST = "127.0.0.1"


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
                    time.sleep(delay)
        raise last_err

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
