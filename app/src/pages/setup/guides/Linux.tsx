import React from 'react';
import CodeBlock from '@/components/setup/CodeBlock';

export const linuxBadges = [
    { label: 'Linux' },
    { label: 'DNS over TLS' },
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

// We now rely on DI; provided context supplies primaryIp, profileId, domain
const buildSystemdResolvedConfig = (ctx: LinuxGuideDeps) => `[Resolve]\nDNS=${ctx.primaryIp}#${ctx.profileId}.${ctx.domain}\nDNSOverTLS=yes`;
const buildDnsmasqConfig = (ctx: LinuxGuideDeps) => `no-resolv\nbogus-priv\nstrict-order\nserver=${ctx.primaryIp}\nadd-cpe-id=${ctx.profileId}`;
const systemdRestartCmd = 'sudo systemctl restart systemd-resolved';
const dnsmasqRestartCmd = 'sudo systemctl restart dnsmasq';

// Factory to build tab definitions with current context
interface LinuxGuideDeps { profileId: string; primaryIp: string; domain: string }

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
