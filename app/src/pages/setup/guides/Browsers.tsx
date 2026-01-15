
// Badges shown at top of guide
export const browsersBadges = [
    { label: "Browsers" },
    { label: "DNS over HTTPS" },
];

/**
 * Build the steps for the Browsers setup guide.
 * Accepts a dnsOverHTTPS URL which is profile-specific (passed from parent panel).
 */
interface BrowsersGuideDeps { dohEndpoint: string }
export const createBrowsersSteps = (
    deps: BrowsersGuideDeps = { dohEndpoint: "https://example.com/dns-query/your-profile-id" }
) => {
    const doh = deps.dohEndpoint;
    // Provide a single step containing a tab UI so RightPanelGuide renders it intact.
    return [
        {
            instruction: (
                <div className="flex flex-col gap-4">
                    <p className="text-sm leading-6 text-[var(--tailwind-colors-slate-200)]">
                        You can configure most browsers to use our service via their "Secure DNS" setting. This change only affects DNS resolutions in the browser, and won't apply system-wide.
                    </p>
                    <BrowserTabs doh={doh} />
                </div>
            )
        }
    ];
};

// Local tab component (lightweight) so we don't add dependencies.
import React from 'react';
import CodeBlock from '@/components/setup/CodeBlock';
import chromeLogo from "@/assets/browsers/google-chrome.svg";
import firefoxLogo from "@/assets/browsers/firefox-browser.svg";
import braveLogo from "@/assets/browsers/brave-browser.svg";
import edgeLogo from "@/assets/browsers/edge-browser.svg";
import safariLogo from "@/assets/browsers/safari-browser.svg";

interface BrowserTabDef {
    key: string;
    label: string;
    logo: string | null; // null means use lucide icon
    content: (doh: string) => React.ReactNode;
}

const browserTabs: BrowserTabDef[] = [
    {
        key: 'chrome',
        label: 'Chrome',
        logo: chromeLogo,
        content: (doh) => (
            <div className="flex flex-col gap-6">
                {[
                    <span>Open <em>Chrome Settings</em>.</span>,
                    <span>Navigate to <em>Privacy and Security</em>.</span>,
                    <span>In the <em>Security / Advanced</em> section, enable <em>Use secure DNS</em>.</span>,
                    <span>At <em>Select DNS Provider</em>, choose <em>Add custom DNS service provider</em>.</span>,
                ].map((text, idx) => (
                    <StepBlock key={idx} number={idx + 1} text={text} />
                ))}
                <StepBlock number={5} text={<span>Enter: <CodeBlock value={doh} /></span>} />
            </div>
        )
    },
    {
        key: 'firefox',
        label: 'Firefox',
        logo: firefoxLogo,
        content: (doh) => (
            <div className="flex flex-col gap-6">
                {[
                    <span>Open <em>Firefox Preferences</em>.</span>,
                    <span>Search for or find <em>Enable DNS over HTTPS</em> under <em>Privacy &amp; Security</em>.</span>,
                    <span>Select either <em>Increased Protection</em> or <em>Max Protection</em>.</span>,
                    <span>Under <em>Choose provider</em>, select <em>Custom</em>.</span>,
                ].map((text, idx) => (
                    <StepBlock key={idx} number={idx + 1} text={text} />
                ))}
                <StepBlock number={5} text={<span>Enter: <CodeBlock value={doh} accent /></span>} />
                <div className="text-xs text-[var(--tailwind-colors-slate-400)] leading-relaxed">
                    You can verify the correct provider is chosen in the box above:<br />
                    <strong>Status:</strong> Active <br />
                    <strong>Provider:</strong> dns.moddns.net
                </div>
            </div>
        )
    },
    {
        key: 'brave',
        label: 'Brave',
        logo: braveLogo,
        content: (doh) => (
            <div className="flex flex-col gap-6">
                {[
                    <span>Open <em>Brave Settings</em>.</span>,
                    <span>Go to the <em>Privacy &amp; Security</em> tab.</span>,
                    <span>Under <em>Security</em>, in the <em>Advanced</em> section, enable <em>Use secure DNS</em>.</span>,
                    <span>At <em>Select DNS Provider</em>, choose <em>Add custom DNS service provider</em>.</span>,
                ].map((text, idx) => (
                    <StepBlock key={idx} number={idx + 1} text={text} />
                ))}
                <StepBlock number={5} text={<span>Enter: <CodeBlock value={doh} /></span>} />
            </div>
        )
    },
    {
        key: 'edge',
        label: 'Edge',
        logo: edgeLogo,
        content: (doh) => (
            <div className="flex flex-col gap-6">
                {[
                    <span>Open <em>Edge Settings</em>.</span>,
                    <span>Access the <em>Privacy, search, and services</em> tab.</span>,
                    <span>In <em>Security</em>, enable <em>Use secure DNS</em> and select <em>Choose a service provider</em>.</span>,
                ].map((text, idx) => (
                    <StepBlock key={idx} number={idx + 1} text={text} />
                ))}
                <StepBlock number={4} text={<span>Enter: <CodeBlock value={doh} accent /></span>} />
            </div>
        )
    },
    {
        key: 'safari',
        label: 'Safari',
        logo: safariLogo,
        content: (_doh) => (
            <div className="flex flex-col gap-6">
                <StepBlock number={1} text={"Safari uses your device's system-wide DNS settings, so please follow our macOS or iOS setup guide to get started."} />
            </div>
        )
    }
];

// Removed local CodeBlock in favor of shared component

const BrowserTabs = ({ doh }: { doh: string }) => {
    const [active, setActive] = React.useState<string>('chrome');
    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-wrap gap-2">
                {browserTabs.map(tab => (
                    <button
                        key={tab.key}
                        onClick={() => setActive(tab.key)}
                        className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs sm:text-sm transition-all duration-300 transform hover:scale-105 active:scale-100 cursor-pointer ${active === tab.key
                            ? 'bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-rdns-600)] text-white'
                            : 'bg-[var(--tailwind-colors-slate-900)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-300)] hover:bg-[var(--tailwind-colors-slate-800)]'
                            }`}
                        type="button"
                    >
                        <img src={tab.logo ?? ''} alt={tab.label} className="w-4 h-4" />
                        <span>{tab.label}</span>
                    </button>
                ))}
            </div>
            <div className="p-4 rounded-md">
                {browserTabs.find(t => t.key === active)?.content(doh)}
            </div>
        </div>
    );
};

// Reusable step block matching RightPanelGuide styling
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

// Default (generic) steps so the panel can render without params
export const browsersSteps = createBrowsersSteps();

const BrowsersGuide = {
    badges: browsersBadges,
    steps: browsersSteps,
};

export default BrowsersGuide;
