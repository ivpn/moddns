// Build a ready-to-paste dnscrypt-proxy.toml snippet from a DoH stamp.
// The dnscrypt-proxy client speaks DoH natively; this registers the stamp as a
// static server, so no native DNSCrypt protocol is involved.
export const buildDnscryptProxyToml = (profileId: string, dohStamp: string) => {
    const serverName = `modDNS-${profileId}`;
    return (
        `server_names = ['${serverName}']\n\n` +
        `[static]\n` +
        `  [static.'${serverName}']\n` +
        `  stamp = '${dohStamp}'`
    );
};
