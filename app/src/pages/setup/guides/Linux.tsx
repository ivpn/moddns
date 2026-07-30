import React from 'react';
import CodeBlock from '@/components/setup/CodeBlock';
import { buildDnscryptProxyToml } from '@/components/setup/dnscryptProxy';
import api from '@/api/api';

export const linuxBadges = [
    { label: 'Linux' },
    { label: 'DNS over TLS' },
    { label: 'DNS over HTTPS' },
];

// STEP block component
// eslint-disable-next-line react-refresh/only-export-components
const StepBlock = ({ number, children }: { number: number; children: React.ReactNode }) => (
    <div className="flex flex-col gap-3">
        <div className="flex items-center gap-2.5">
            <div className="text-sm text-[var(--tailwind-colors-slate-200)] leading-5">STEP {number}</div>
        </div>
        <div className="text-sm text-[var(--tailwind-colors-slate-50)] leading-6">
            {children}
        </div>
    </div>
);

interface TabDef { key: string; label: string; content: React.ReactNode }

// We now rely on DI; provided context supplies primaryIp, profileId, domain, optional ipv6
const buildSystemdResolvedConfig = (ctx: LinuxGuideDeps) => {
    const sni = `${ctx.profileId}.${ctx.domain}`;
    // DoT transport works over IPv6; append the anycast IPv6 server when configured.
    const servers = ctx.ipv6 ? `${ctx.primaryIp}#${sni} ${ctx.ipv6}#${sni}` : `${ctx.primaryIp}#${sni}`;
    return `[Resolve]\nDNS=${servers}\nDNSOverTLS=yes\nDomains=~.`;
};
const buildDnsmasqConfig = (ctx: LinuxGuideDeps) => `no-resolv\nbogus-priv\nstrict-order\nserver=${ctx.primaryIp}\nadd-cpe-id=${ctx.profileId}`;
const systemdRestartCmd = 'sudo systemctl restart systemd-resolved';
const dnsmasqRestartCmd = 'sudo systemctl restart dnsmasq';
const dnscryptRestartCmd = 'sudo systemctl restart dnscrypt-proxy';

// Factory to build tab definitions with current context
interface LinuxGuideDeps { profileId: string; primaryIp: string; domain: string; ipv6?: string }

// dnscrypt-proxy consumes the modDNS DoH stamp (proto 0x02) natively — this is
// DoH via dnscrypt-proxy, not the native DNSCrypt protocol. The Linux guide does
// not otherwise fetch stamps, so this tab fetches the DoH stamp itself, mirroring
// the fetch/loading/error handling in Routers' StampsTab.
// eslint-disable-next-line react-refresh/only-export-components
const LinuxDnscryptProxyTab = ({ deps }: { deps: LinuxGuideDeps }) => {
    const [doh, setDoh] = React.useState('');
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);

    const fetchStamp = React.useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await api.Client.dnsStampsApi.apiV1DnsstampPost({
                profile_id: deps.profileId,
                device_id: '',
            });
            setDoh(res.data.doh ?? '');
        } catch {
            setError('Could not generate the DoH stamp. Try again.');
            setDoh('');
        } finally {
            setLoading(false);
        }
    }, [deps.profileId]);

    React.useEffect(() => { fetchStamp(); }, [fetchStamp]);

    return (
        <div className="flex flex-col gap-6">
            <div className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">Linux dnscrypt-proxy - DNS-over-HTTPS</div>
            <div className="flex flex-col gap-6">
                <StepBlock number={1}>
                    Install <strong>dnscrypt-proxy</strong> from your distribution's package manager
                    (e.g. <code className="font-mono text-xs">sudo apt install dnscrypt-proxy</code>), or download the
                    official binary from{' '}
                    <a
                        href="https://github.com/DNSCrypt/dnscrypt-proxy/releases"
                        target="_blank"
                        rel="noreferrer"
                        className="!underline !text-[var(--tailwind-colors-slate-300)]"
                    >
                        the dnscrypt-proxy releases
                    </a>.
                </StepBlock>
                <StepBlock number={2}>
                    Edit <code className="font-mono text-xs">dnscrypt-proxy.toml</code> to consume the modDNS DoH stamp:
                    <div data-testid="dnscrypt-proxy-config">
                        {error ? (
                            <div
                                role="alert"
                                className="mt-2 flex items-center justify-between rounded border border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-900)] px-3 py-2 text-sm text-[var(--tailwind-colors-slate-200)]"
                            >
                                <span>{error}</span>
                                <button
                                    type="button"
                                    onClick={fetchStamp}
                                    className="text-xs px-2 py-1 rounded-md bg-[var(--tailwind-colors-rdns-600)] text-white hover:opacity-90"
                                >
                                    Retry
                                </button>
                            </div>
                        ) : loading || !doh ? (
                            <div className="mt-2 h-24 rounded border border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-900)] animate-pulse" />
                        ) : (
                            <CodeBlock value={buildDnscryptProxyToml(deps.profileId, doh)} />
                        )}
                    </div>
                </StepBlock>
                <StepBlock number={3}>
                    Point your system resolver at dnscrypt-proxy's <code className="font-mono text-xs">listen_addresses</code>
                    {' '}(default <code className="font-mono text-xs">127.0.0.1:53</code>), then restart the service:
                    <CodeBlock value={dnscryptRestartCmd} />
                </StepBlock>
            </div>
        </div>
    );
};

function buildTabs(deps: LinuxGuideDeps): TabDef[] {
    const systemdResolvedConfig = buildSystemdResolvedConfig(deps);
    const dnsmasqConfig = buildDnsmasqConfig(deps);
    return [
        {
            key: 'systemd-resolved',
            label: 'systemd-resolved',
            content: (
                <div className="flex flex-col gap-6">
                    <div className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">Linux systemd-resolved - DNS-over-TLS</div>
                    <div className="flex flex-col gap-6">
                        <StepBlock number={1}>
                            On the modDNS website, go to <span className="font-medium">Settings &gt; Advanced Settings</span>, and set <span className="font-medium">DNSSEC OK (DO) bit</span> to <span className="font-medium">Disable</span>
                        </StepBlock>
                        <StepBlock number={2}>
                            Edit <code className="font-mono text-xs">/etc/systemd/resolved.conf</code>:
                            <CodeBlock value={systemdResolvedConfig} />
                            <div className="mt-3 text-xs text-[var(--tailwind-colors-slate-300)] leading-relaxed">
                                <strong>Domains=~.</strong> routes all DNS queries through this server. This is required when running alongside a VPN client (e.g. IVPN) — without it, queries can fall back to other resolvers and break DNS connectivity.
                            </div>
                        </StepBlock>
                        <StepBlock number={3}>
                            Restart the systemd-resolved service: <code className="font-mono text-xs">{systemdRestartCmd}</code>
                            <CodeBlock value={systemdRestartCmd} />
                        </StepBlock>
                    </div>
                </div>
            )
        },
        {
            key: 'dnsmasq',
            label: 'dnsmasq',
            content: (
                <div className="flex flex-col gap-6">
                    <div className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">Linux dnsmasq - DNS-over-TLS</div>
                    <div className="flex flex-col gap-6">
                        <StepBlock number={1}>
                            On the modDNS website, go to <span className="font-medium">Settings &gt; Advanced Settings</span>, and set <span className="font-medium">DNSSEC OK (DO) bit</span> to <span className="font-medium">Disable</span>
                        </StepBlock>
                        <StepBlock number={2}>
                            Edit <code className="font-mono text-xs">dnsmasq.conf</code>:
                            <CodeBlock value={dnsmasqConfig} />
                        </StepBlock>
                        <StepBlock number={3}>
                            Restart the dnsmasq service: <code className="font-mono text-xs">{dnsmasqRestartCmd}</code>
                            <CodeBlock value={dnsmasqRestartCmd} />
                        </StepBlock>
                    </div>
                </div>
            )
        },
        {
            key: 'dnscrypt-proxy',
            label: 'dnscrypt-proxy',
            content: <LinuxDnscryptProxyTab deps={deps} />
        }
    ];
}

// eslint-disable-next-line react-refresh/only-export-components
const LinuxTabs = ({ deps }: { deps: LinuxGuideDeps }) => {
    const [active, setActive] = React.useState('systemd-resolved');
    const tabs = React.useMemo(() => buildTabs(deps), [deps]);
    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-wrap gap-2">
                {tabs.map(tab => (
                    <button
                        key={tab.key}
                        onClick={() => setActive(tab.key)}
                        type="button"
                        className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs sm:text-sm transition-all duration-300 transform hover:scale-105 active:scale-100 cursor-pointer ${active === tab.key
                            ? 'bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-rdns-600)] text-white'
                            : 'bg-[var(--tailwind-colors-slate-900)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-300)] hover:bg-[var(--tailwind-colors-slate-800)]'
                            }`}
                    >
                        <span>{tab.label}</span>
                    </button>
                ))}
            </div>
            <div className="p-4">
                {tabs.find(t => t.key === active)?.content}
            </div>
        </div>
    );
};

export const createLinuxSteps = (deps: LinuxGuideDeps) => ([
    {
        instruction: (
            <div className="flex flex-col gap-4">
                <LinuxTabs deps={deps} />
            </div>
        )
    }
]);

// No static steps now; consumer must inject deps.
export const buildLinuxGuide = (deps: LinuxGuideDeps) => ({
    badges: linuxBadges,
    steps: createLinuxSteps(deps)
});

export default {
    badges: linuxBadges,
    createLinuxSteps,
    buildLinuxGuide,
};
