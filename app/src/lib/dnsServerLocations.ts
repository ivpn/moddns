export interface DnsServerLocation {
    prefix: string;
    city: string;
    hostname: string;
    ipv4?: string;
    ipv6?: string;
}

const nonEmpty = (value: string | undefined): string | undefined => {
    const trimmed = value?.trim();
    return trimmed ? trimmed : undefined;
};

/**
 * Parses VITE_DNS_SERVER_LOCATIONS: comma-separated `prefix|City|ipv4|ipv6` entries.
 * `|` is the field separator because IPv6 literals contain `:`; the older
 * `prefix:City` form is still accepted and yields no addresses.
 */
export function parseDnsServerLocations(raw: string | undefined, domain: string): DnsServerLocation[] {
    return (raw ?? '')
        .split(',')
        .map(entry => entry.trim())
        .filter(Boolean)
        .map(entry => {
            const fields = entry.includes('|') ? entry.split('|') : entry.split(':');
            const [prefixField, cityField, ipv4Field, ipv6Field] = fields;
            const prefix = prefixField.trim();
            return {
                prefix,
                city: nonEmpty(cityField) ?? prefix,
                hostname: `${prefix}.${domain}`,
                ipv4: nonEmpty(ipv4Field),
                ipv6: nonEmpty(ipv6Field),
            };
        });
}

/** First non-empty item of a comma-separated address list (VITE_DNS_SERVER_IP*_ADDRESSES). */
export function firstAddress(raw: string | undefined): string | undefined {
    return (raw ?? '')
        .split(',')
        .map(item => item.trim())
        .find(Boolean);
}
