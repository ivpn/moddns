// Router setup commands shown on Setup > Routers. No imports and erasable TypeScript only:
// the backend E2E suite renders them with Node type stripping (tests/libs/openwrt.py).

export interface RouterCommandDeps {
    dohEndpoint: string;      // https://<dnsServerDomain>/dns-query/<profileId>
    anycastIpv4: string;      // primary IPv4 from env list
    anycastIpv6?: string;     // anycast IPv6 from env list; shown only when configured
    dnsServerDomain: string;  // e.g. dns.moddns.net (from env)
}

export const buildMikrotikCommands = ({ dohEndpoint, anycastIpv4, anycastIpv6, dnsServerDomain }: RouterCommandDeps) => (
    `/ip dns set servers=""\n` +
    `/ip dns static add name=${dnsServerDomain} address=${anycastIpv4} type=A\n` +
    (anycastIpv6 ? `/ip dns static add name=${dnsServerDomain} address=${anycastIpv6} type=AAAA\n` : ``) +
    `/ip dns set use-doh-server="${dohEndpoint}" verify-doh-cert=yes\n` +
    `/ip dns set allow-remote-requests=yes`
);

// No bootstrap_dns: https-dns-proxy resolves the DoH hostname with plain DNS, which
// modDNS does not answer, so the package's default public resolvers do it.
export const buildOpenWrtCommands = ({ dohEndpoint }: Pick<RouterCommandDeps, 'dohEndpoint'>) => (
    `# OpenWrt 24.10 and older use opkg package manager\n` +
    `opkg update\n` +
    `opkg install https-dns-proxy\n\n` +
    `# OpenWrt 25.12 and newer use apk package manager\n` +
    `apk update\n` +
    `apk add https-dns-proxy\n\n` +
    `while uci -q delete https-dns-proxy.@https-dns-proxy[0]; do :; done\n` +
    `uci set https-dns-proxy.dns="https-dns-proxy"\n` +
    `uci set https-dns-proxy.dns.resolver_url="${dohEndpoint}"\n` +
    `uci set https-dns-proxy.dns.listen_addr="127.0.0.1"\n` +
    `uci set https-dns-proxy.dns.listen_port="5053"\n` +
    `uci commit https-dns-proxy\n` +
    `service https-dns-proxy restart`
);
