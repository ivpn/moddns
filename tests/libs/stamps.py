"""Fetch the product's sdns:// stamps and adapt them for a client container."""
from __future__ import annotations

from contextlib import contextmanager

import dnsstamps
from dnsstamps import Protocol

import moddns.api as api
import moddns.api_client as client
import moddns.configuration as api_config
from moddns import RequestsDNSStampReq

from libs.session import ProfileSession

PROTOCOLS = ("doh", "dot", "doq")


@contextmanager
def stamps_api(user: ProfileSession):
    """Cookie-authenticated DNSStampsApi (not wrapped by ProfileSession)."""
    api_conf = api_config.Configuration(host=user.config.DNS_API_ADDR)
    with client.ApiClient(api_conf) as api_client:
        api_client.default_headers["Cookie"] = user.cookie
        yield api.DNSStampsApi(api_client)


def fetch_stamps(user: ProfileSession, profile_id: str, device_id: str = "") -> dict[str, str]:
    """The three per-profile stamps exactly as POST /api/v1/dnsstamp hands them out."""
    with stamps_api(user) as stamps:
        resp = stamps.api_v1_dnsstamp_post(
            body=RequestsDNSStampReq(profile_id=profile_id, device_id=device_id)
        )
    return {"doh": resp.doh, "dot": resp.dot, "doq": resp.doq}


def with_address(stamp: str, address: str) -> str:
    """Re-encode ``stamp`` so its address field points at ``address``.

    Stamps carry the resolver's IP (production: the anycast address; this stack:
    the host loopback from config/api.env). A client running *in a container*
    must dial the proxy's compose address instead, so only the address changes —
    hostname, path, port and properties are kept, which is what carries the
    profile to the proxy.
    """
    p = dnsstamps.parse(stamp)
    port = p.address.rsplit(":", 1)[1] if ":" in p.address else None
    new_address = f"{address}:{port}" if port else address
    if p.protocol == Protocol.DOH:
        return dnsstamps.create_doh(new_address, p.hashes, p.hostname, p.path, options=p.options)
    if p.protocol == Protocol.DOT:
        return dnsstamps.create_dot(new_address, p.hashes, p.hostname, options=p.options)
    if p.protocol == Protocol.DOQ:
        return dnsstamps.create_doq(new_address, p.hashes, p.hostname, options=p.options)
    raise ValueError(f"unsupported stamp protocol: {p.protocol}")
