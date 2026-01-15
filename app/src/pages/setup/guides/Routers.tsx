import React from 'react';
import CodeBlock from '@/components/setup/CodeBlock';

export const routersBadges = [
    { label: 'Routers' },
    { label: 'DNS over HTTPS' },
    { label: 'DNS over TLS' },

];

export interface RoutersGuideDeps {
    dohEndpoint: string;      // https://<dnsServerDomain>/dns-query/<profileId>
    anycastIpv4: string;      // primary IPv4 from env list
    dnsServerDomain: string;  // e.g. dns.moddns.net (from env)
    dotHostname: string;      // <profileId>.<dnsServerDomain>
}

interface RouterTabDef {
    key: string;
    label: string;
    content: React.ReactNode;
}

const SectionLabel = ({ children }: { children: React.ReactNode }) => (
    <div className="text-xs font-semibold tracking-[0.08em] uppercase text-[var(--tailwind-colors-slate-300)]">
        {children}
    </div>
);

const SectionDivider = () => (
    <div className="h-px w-full bg-[var(--tailwind-colors-slate-700)]" />
);

const StepBlock = ({ number, text }: { number: number; text: React.ReactNode }) => (
    <div className="flex flex-col gap-3">
        <div className="flex items-center gap-2.5">
            <div className="text-sm text-[var(--tailwind-colors-slate-200)] leading-5 font-['Roboto_Flex-Regular',Helvetica]">
                STEP {number}
            </div>
        </div>
        <div className="text-sm text-[var(--tailwind-colors-slate-50)] leading-6 font-['Roboto_Flex-Regular',Helvetica]">
            {text}
        </div>
    </div>
);

const buildMikrotikCommands = ({ dohEndpoint, anycastIpv4, dnsServerDomain }: RoutersGuideDeps) => (
    `/ip dns set servers=""\n` +
    `/ip dns static add name=${dnsServerDomain} address=${anycastIpv4} type=A\n` +
    `/ip dns set use-doh-server="${dohEndpoint}" verify-doh-cert=yes\n` +
    `/ip dns set allow-remote-requests=yes`
);

const buildOpenWrtCommands = ({ dohEndpoint, anycastIpv4 }: RoutersGuideDeps) => (
    `opkg update\n` +
    `opkg install https-dns-proxy\n\n` +
    `while uci -q delete https-dns-proxy.@https-dns-proxy[0]; do :; done\n` +
    `uci set https-dns-proxy.dns="https-dns-proxy"\n` +
    `uci set https-dns-proxy.dns.bootstrap_dns="${anycastIpv4}"\n` +
    `uci set https-dns-proxy.dns.resolver_url="${dohEndpoint}"\n` +
    `uci set https-dns-proxy.dns.listen_addr="127.0.0.1"\n` +
    `uci set https-dns-proxy.dns.listen_port="5053"\n` +
    `uci commit https-dns-proxy\n` +
    `service https-dns-proxy restart`
);

const buildRouterTabs = (deps: RoutersGuideDeps): RouterTabDef[] => [
    {
        key: 'mikrotik',
        label: 'Mikrotik Router OS',
        content: (
            <div className="flex flex-col gap-6">
                <StepBlock number={1} text={<span>Access the device’s command-line interface, and enter the following commands:</span>} />
                <div>
                    <CodeBlock value={buildMikrotikCommands(deps)} />
                </div>
            </div>
        )
    },
    {
        key: 'pfsense',
        label: 'pfSense',
        content: (
            <div className="flex flex-col gap-6">
                <SectionLabel>System &gt; General Setup &gt; DNS Server Settings:</SectionLabel>
                <div className="flex flex-col gap-6">
                    <StepBlock number={1} text={<span><strong>DNS Servers:</strong> clear all entries from <span className="font-medium">DNS Servers</span></span>} />
                    <StepBlock
                        number={2}
                        text={(
                            <span>
                                <strong>DNS Servers:</strong> add <CodeBlock inline noWrap value={deps.anycastIpv4} /> to <span className="font-medium">Address</span> and{' '}
                                <CodeBlock inline noWrap value={deps.dotHostname} /> to <span className="font-medium">Hostname</span>, repeat for each IP address, hostname is always the same
                            </span>
                        )}
                    />
                    <StepBlock
                        number={3}
                        text={<span><strong>DNS Server Override:</strong> uncheck <span className="font-medium">Allow DNS server list to overridden by DHCP...</span></span>}
                    />
                    <StepBlock
                        number={4}
                        text={<span><strong>DNS Resolution Behavior:</strong> select <span className="font-medium">Use local DNS (127.0.0.1), ignore remote DNS servers</span></span>}
                    />
                    <StepBlock number={5} text={<span className="font-medium">Save</span>} />
                </div>

                <SectionDivider />

                <SectionLabel>Services &gt; DNS Resolver &gt; General Settings:</SectionLabel>
                <div className="flex flex-col gap-6">
                    <StepBlock number={1} text={<span><strong>DNSSEC:</strong> uncheck <span className="font-medium">Enable DNSSEC support</span></span>} />
                    <StepBlock
                        number={2}
                        text={<span><strong>DNS Query Forwarding:</strong> check <span className="font-medium">Enable Forwarding Mode</span>, and <span className="font-medium">Use SSL/TLS for outgoing DNS queries to Forwarding Servers</span></span>}
                    />
                    <StepBlock number={3} text={<span className="font-medium">Save and Apply</span>} />
                </div>
            </div>
        )
    },
    {
        key: 'opnsense',
        label: 'OPNsense',
        content: (
            <div className="flex flex-col gap-6">
                <SectionLabel>System &gt; Configuration &gt; Backups:</SectionLabel>
                <StepBlock number={1} text={<span><strong>Download configuration:</strong> known good</span>} />

                <SectionDivider />

                <SectionLabel>Services &gt; Unbound DNS &gt; General:</SectionLabel>
                <div className="flex flex-col gap-6">
                    <StepBlock number={1} text={<span><strong>Enable DNSSEC Support:</strong> unchecked</span>} />
                    <StepBlock number={2} text={<span><span className="font-medium">Apply</span>, if required</span>} />
                </div>

                <SectionDivider />

                <SectionLabel>Services &gt; Unbound DNS &gt; DNS over TLS:</SectionLabel>
                <div className="flex flex-col gap-6">
                    <StepBlock number={1} text={<span><strong>Use System Nameservers:</strong> unchecked</span>} />
                    <StepBlock number={2} text={<span>Click <strong>+</strong> to add a server</span>} />
                    <StepBlock number={3} text={<span><strong>Enabled:</strong> checked</span>} />
                    <StepBlock number={4} text={<span><strong>Domain:</strong> leave this field empty</span>} />
                    <StepBlock number={5} text={<span><strong>Server IP:</strong> <CodeBlock inline noWrap value={deps.anycastIpv4} /></span>} />
                    <StepBlock number={6} text={<span><strong>Server Port:</strong> <CodeBlock inline noWrap value="853" /></span>} />
                    <StepBlock number={7} text={<span><strong>Forward First:</strong> unchecked</span>} />
                    <StepBlock number={8} text={<span><strong>Verify CN:</strong> <CodeBlock inline noWrap value={deps.dotHostname} /></span>} />
                    <StepBlock number={9} text={<span><strong>Description:</strong> optional</span>} />
                    <StepBlock number={10} text={<span className="font-medium">Save and Apply</span>} />
                </div>

                <div className="text-xs text-[var(--tailwind-colors-slate-400)] leading-relaxed">
                    To add more servers, only the <strong>Server IP</strong> address above changes, hostname is always the same.
                </div>

                <SectionDivider />

                <SectionLabel>System &gt; Settings &gt; General &gt; Networking:</SectionLabel>
                <div className="flex flex-col gap-6">
                    <StepBlock number={1} text={<span><strong>DNS servers:</strong> clear all entries</span>} />
                    <StepBlock
                        number={2}
                        text={<span><strong>DNS server options:</strong> uncheck <span className="font-medium">Allow DNS server list to overridden by DHCP...</span></span>}
                    />
                    <StepBlock number={3} text={<span className="font-medium">Save</span>} />
                </div>
            </div>
        )
    },
    {
        key: 'openwrt',
        label: 'OpenWrt',
        content: (
            <div className="flex flex-col gap-6">
                <StepBlock number={1} text={<span>Access the command line:</span>} />
                <div>
                    <CodeBlock value={buildOpenWrtCommands(deps)} />
                </div>
            </div>
        )
    }
];

const RouterTabs = ({ deps }: { deps: RoutersGuideDeps }) => {
    const [active, setActive] = React.useState<string>('mikrotik');
    const routerTabs = React.useMemo(() => buildRouterTabs(deps), [deps]);
    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-wrap gap-2">
                {routerTabs.map(tab => (
                    <button
                        key={tab.key}
                        onClick={() => setActive(tab.key)}
                        className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs sm:text-sm transition-all duration-300 transform hover:scale-105 active:scale-100 cursor-pointer ${active === tab.key
                            ? 'bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-rdns-600)] text-white'
                            : 'bg-[var(--tailwind-colors-slate-900)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-300)] hover:bg-[var(--tailwind-colors-slate-800)]'
                            }`}
                        type="button"
                    >
                        <span>{tab.label}</span>
                    </button>
                ))}
            </div>
            <div className="p-4 rounded-md">
                {routerTabs.find(t => t.key === active)?.content}
            </div>
        </div>
    );
};

export const createRoutersSteps = (deps: RoutersGuideDeps) => {
    return [
        {
            instruction: (
                <div className="flex flex-col gap-4">
                    <p className="text-sm leading-6 text-[var(--tailwind-colors-slate-200)]">
                        Select your router/firewall platform below.
                    </p>
                    <RouterTabs deps={deps} />
                </div>
            )
        }
    ];
};

// Default (generic) steps so the panel can render without injected deps.
export const routersSteps = createRoutersSteps({
    dohEndpoint: 'https://example.com/dns-query/your-profile-id',
    anycastIpv4: '0.0.0.0',
    dnsServerDomain: 'example.com',
    dotHostname: 'your-profile-id.example.com'
});

const RoutersGuide = {
    badges: routersBadges,
    steps: routersSteps,
};

export default RoutersGuide;
