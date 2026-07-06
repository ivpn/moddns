import { CodeBlock } from '@/components/setup/CodeBlock';

// eslint-disable-next-line react-refresh/only-export-components
export const vpnAppsBadges = [
    { label: 'VPN apps' },
    { label: 'DNS over HTTPS' },
    { label: 'DNS over TLS' },
];

export interface VpnAppsGuideDeps {
    dohEndpoint: string;     // https://<dnsServerDomain>/dns-query/<profileId>
    dotEndpoint: string;     // <profileId>.<dnsServerDomain>
    primaryIp: string;       // primary IPv4 from env list
    onPlatformChange?: (platform: string) => void;
}

// eslint-disable-next-line react-refresh/only-export-components
export const createVpnAppsSteps = (deps: VpnAppsGuideDeps) => [
    {
        instruction: (
            <div className="flex flex-col gap-4">
                <p className="text-sm leading-6 text-[var(--shadcn-ui-app-foreground)]">
                    To use modDNS when connected to a VPN, enable Custom DNS in the VPN app settings.
                    DoH/DoT must be supported with the URL specified in the VPN app —
                    adding a plain IPv4 DNS address is not sufficient.
                </p>
                <div className="flex flex-col gap-2 p-4 bg-[var(--tailwind-colors-rdns-600)]/10 rounded-lg border border-[var(--tailwind-colors-rdns-600)]/30">
                    <p className="text-sm leading-6 text-[var(--shadcn-ui-app-foreground)]">
                        <strong>Note:</strong> Android does not support proper DoH/DoT integration
                        in VPN apps. Android's Private DNS overrides VPN DNS settings and should
                        be used as a setup method — switch to the{' '}
                        {deps.onPlatformChange ? (
                            <button
                                type="button"
                                onClick={() => deps.onPlatformChange?.('Android')}
                                className="underline underline-offset-2 text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--tailwind-colors-rdns-600)] rounded-sm cursor-pointer"
                            >
                                Android
                            </button>
                        ) : (
                            <span>Android</span>
                        )}{' '}
                        guide.
                    </p>
                </div>
                <p className="text-sm leading-6 text-[var(--shadcn-ui-app-muted-foreground)]">
                    Steps below are for IVPN apps. For other VPN services, consult the relevant
                    documentation.
                </p>
            </div>
        ),
    },
    {
        step: 1,
        instruction: (
            <span>
                Open the IVPN app and navigate to <strong>Settings</strong>.
            </span>
        ),
    },
    {
        step: 2,
        instruction: (
            <span>
                Select <strong>Custom DNS</strong>, and enable it.
            </span>
        ),
    },
    {
        step: 3,
        instruction: (
            <span>
                Under <strong>DNS Server / IP Address</strong>, enter:
                <div className="mt-2"><CodeBlock noWrap value={deps.primaryIp} className="w-full" /></div>
            </span>
        ),
    },
    {
        step: 4,
        instruction: (
            <span>
                Under <strong>DNS over HTTPS / DNS over TLS URL</strong>, enter one of the following:
                <div className="mt-3 flex flex-col gap-3">
                    <div className="flex flex-col gap-1">
                        <div className="text-xs font-semibold tracking-[0.08em] uppercase text-[var(--shadcn-ui-app-muted-foreground)]">
                            DNS over HTTPS
                        </div>
                        <CodeBlock noWrap value={deps.dohEndpoint} className="w-full" />
                    </div>
                    <div className="flex flex-col gap-1">
                        <div className="text-xs font-semibold tracking-[0.08em] uppercase text-[var(--shadcn-ui-app-muted-foreground)]">
                            DNS over TLS
                        </div>
                        <CodeBlock noWrap value={deps.dotEndpoint} className="w-full" />
                    </div>
                </div>
            </span>
        ),
    },
    {
        step: 5,
        instruction: (
            <span>
                Save and reconnect to the VPN.
            </span>
        ),
    },
];

// Default (generic) steps so the panel can render without injected deps.
// eslint-disable-next-line react-refresh/only-export-components
export const vpnAppsSteps = createVpnAppsSteps({
    dohEndpoint: 'https://example.com/dns-query/your-profile-id',
    dotEndpoint: 'your-profile-id.example.com',
    primaryIp: '0.0.0.0',
});

const VpnAppsGuide = {
    badges: vpnAppsBadges,
    steps: vpnAppsSteps,
};

export default VpnAppsGuide;
