import React, { type JSX, useState, useRef, useEffect } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ArrowLeft, ChevronDown } from "lucide-react";
import modDNSLogo from '@/assets/logos/modDNS.svg';
import AuthFooter from "@/components/auth/AuthFooter";

interface FAQItemProps {
    question: string;
    answer: string | JSX.Element;
    globalToggleSignal?: number; // Used to trigger expand/collapse from parent
    globalToggleState?: boolean; // What state to set when signal changes
}

function FAQItem({ question, answer, globalToggleSignal, globalToggleState }: FAQItemProps) {
    const [isExpanded, setIsExpanded] = useState(false);
    const [height, setHeight] = useState<string>('0px');
    const contentRef = useRef<HTMLDivElement>(null);
    const [lastSignal, setLastSignal] = useState(0);

    // React to global toggle signals
    useEffect(() => {
        if (globalToggleSignal && globalToggleSignal !== lastSignal) {
            setIsExpanded(globalToggleState || false);
            setLastSignal(globalToggleSignal);
        }
    }, [globalToggleSignal, globalToggleState, lastSignal]);

    useEffect(() => {
        if (contentRef.current) {
            setHeight(isExpanded ? `${contentRef.current.scrollHeight}px` : '0px');
        }
    }, [isExpanded]);

    const handleToggle = () => {
        setIsExpanded(!isExpanded);
    };

    return (
        <div className="mb-4 border border-[var(--shadcn-ui-app-border)] rounded-lg overflow-hidden bg-[var(--shadcn-ui-app-card)]">
            <button
                className="w-full p-4 text-left hover:bg-[var(--shadcn-ui-app-muted)] transition-colors duration-200 flex items-center justify-between"
                onClick={handleToggle}
            >
                <h3 className="text-xl font-semibold text-[var(--shadcn-ui-app-foreground)] pr-4">
                    {question}
                </h3>
                <span className="ml-2 flex-shrink-0">
                    <ChevronDown
                        className={`h-5 w-5 text-[var(--shadcn-ui-app-muted-foreground)] transition-transform duration-200 ${isExpanded ? 'rotate-180' : ''}`}
                    />
                </span>
            </button>
            <div
                ref={contentRef}
                style={{ height }}
                className="overflow-hidden transition-all duration-300 ease-in-out"
            >
                <div className="p-4 pt-0 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                    {typeof answer === 'string' ? (
                        <p>{answer}</p>
                    ) : (
                        answer
                    )}
                </div>
            </div>
        </div>
    );
}

interface FAQSectionProps {
    title: string;
    children: React.ReactNode;
    globalToggleSignal?: number;
    globalToggleState?: boolean;
}

function FAQSection({ title, children, globalToggleSignal, globalToggleState }: FAQSectionProps) {
    const enhancedChildren = React.Children.map(children, (child) => {
        if (React.isValidElement(child) && child.type === FAQItem) {
            return React.cloneElement(child as React.ReactElement<FAQItemProps>, {
                globalToggleSignal,
                globalToggleState,
            });
        }
        return child;
    });

    return (
        <div className="mb-8">
            <h2 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] mb-6 border-b border-[var(--shadcn-ui-app-border)] pb-2">
                {title}
            </h2>
            {enhancedChildren}
        </div>
    );
}

const FAQ_LAST_UPDATED = 'February 11, 2026';

export default function FAQ(): JSX.Element {
    const navigate = useNavigate();
    const location = useLocation();
    const hasHistory = location.key !== "default";
    const [toggleSignal, setToggleSignal] = useState(0);
    const [toggleState, setToggleState] = useState(false);

    const toggleAllFAQs = () => {
        setToggleState(!toggleState);
        setToggleSignal(prev => prev + 1);
    };

    const supportedProtocols = (
        <ul className="list-disc pl-5 space-y-1">
            <li>DNS-over-HTTPS (DoH) - Port 443</li>
            <li>DNS-over-TLS (DoT) - Port 853</li>
            <li>DNS-over-QUIC (DoQ) - Port 853</li>
        </ul>
    );

    const howToCreateProfile = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>At the top-right of the page, click the current profile button</li>
            <li>Click the '+ Create profile' button</li>
            <li>Give the profile a name, and click the 'Create profile' button</li>
        </ol>
    );

    const howToRenameProfile = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>At the top-right of the page, click the current profile button</li>
            <li>Click to select the profile to rename</li>
            <li>Click the gear icon to the right of the profile name</li>
            <li>After entering a new name, the 'Save' button appears</li>
            <li>Click 'Save', and a confirmation appears at the bottom of the page</li>
        </ol>
    );

    const howToDeleteProfile = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>At the top-right of the page, click the current profile button</li>
            <li>Click to select the profile to delete</li>
            <li>Click the gear icon to the right of the profile name</li>
            <li><strong>Important:</strong> This confirmation step is permanent, and the profile, associated logs, and settings cannot be restored</li>
        </ol>
    );

    const blocklistsInstructions = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>Go to modDNS dashboard</li>
            <li>Navigate to Blocklists</li>
            <li>Enable or disable blocklists individually, or choose Enable listed after filtering lists</li>
            <li>Changes take effect immediately - note that your local DNS cache may still hold old records for some time</li>
        </ol>
    );

    const howToBlockAllQueries = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>Click the 'Settings' tab on the left side of the page</li>
            <li>Under 'BLOCKLISTS', toggle the 'Default rule' from Allow to Block</li>
        </ol>
    );

    const howToAddCustomRule = (
        <div className="space-y-2">
            <p>Click the 'Custom Rules' tab on the left side of the page. Two tabs are available:</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>Denylist entries</strong> help block specific domains not covered by blocklists</li>
                <li><strong>Allowlist</strong> specifies domains that should be allowed even if they appear on active blocklists</li>
            </ul>
            <p>Enter a domain or IP address in the text entry field (for example <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">facebook.com</code>), then click the green '+ Add' button.</p>
        </div>
    );

    const customRulesSubdomainsInfo = (
        <div className="space-y-2">
            <p>By default, when you add a plain domain like <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">facebook.com</code> to your denylist or allowlist, it is automatically stored as <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.facebook.com</code>, meaning the rule applies to the domain and all its subdomains (e.g. <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">www.facebook.com</code>, <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">m.facebook.com</code>).</p>
            <p>This behavior is controlled by the <strong>'Subdomains in custom rules'</strong> setting under 'Settings' &gt; 'CUSTOM RULES':</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>Include</strong> (default): Plain domains are automatically expanded to include subdomains (<code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">facebook.com</code> → <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.facebook.com</code>)</li>
                <li><strong>Exact</strong>: Domains are stored exactly as entered. To include subdomains, you must explicitly use a wildcard (<code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.facebook.com</code>)</li>
            </ul>
            <p>This setting only affects new rules. Existing rules are not changed when you toggle the setting. IP addresses are not affected by this setting.</p>
        </div>
    );

    const customRulesSupportedInputs = (
        <div className="space-y-2">
            <p>Custom Rules support two types of entries:</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>Domains</strong> (including wildcards, like <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.example.com</code>)</li>
                <li><strong>IP addresses</strong> (single IPv4/IPv6 addresses)</li>
            </ul>
        </div>
    );

    const rulePrecedenceInfo = (
        <div className="space-y-2">
            <p>When multiple rules could apply, modDNS uses a simple precedence model:</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>Allowlist wins over blocking.</strong> If an allowlist entry matches, the request is allowed even if it would otherwise be blocked.</li>
                <li><strong>Otherwise, blocking applies.</strong> If there is no allow match but a denylist/service/blocklist match exists, the request is blocked.</li>
                <li><strong>Otherwise, the default rule applies</strong> (your profile's Default rule setting).</li>
            </ul>
            <p>This applies consistently whether the match comes from domains or IP addresses.</p>
        </div>
    );

    const wildcardRules = (
        <div className="space-y-2">
            <p><code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.ads-example.com</code> or <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">.ads-example.com</code> matches <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads-example.com</code> plus any subdomain, such as <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads1.ads-example.com</code> and <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads2.ads-example.com</code>.</p>
            <p><code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads.*</code> matches any domain starting with that label (for example <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads.com</code>, <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads.co.uk</code>, <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads.us</code>), but does not match subdomains such as <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">www.ads.com</code>.</p>
            <p><code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*ads*</code> matches any domain that contains that fragment anywhere, including <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads.com</code>, <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">www.ads.com</code>, <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">exampleads.com</code>, and <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">cdn.exampleads.net</code>.</p>
            <p>There are two separate subdomain settings in 'Settings':</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>'BLOCKLISTS' &gt; 'Subdomains in blocklists'</strong> (default: Block) controls whether subdomains of domains on enabled blocklists are also blocked. For example, if a blocklist contains <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads-example.com</code>, this setting determines whether <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">ads1.ads-example.com</code> is also blocked.</li>
                <li><strong>'CUSTOM RULES' &gt; 'Subdomains in custom rules'</strong> (default: Include) controls whether new custom rules automatically include subdomains. When set to Include, adding <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">example.com</code> is stored as <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.example.com</code>, matching the domain and all its subdomains. When set to Exact, the rule matches only the exact domain entered.</li>
            </ul>
            <p>You can always use explicit wildcards (e.g. <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.example.com</code>) regardless of the setting.</p>
        </div>
    );

    const dotRules = (
        <ul className="list-disc pl-5 space-y-1">
            <li>A leading dot is treated the same as a prefix wildcard (for example <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">.ads-example.com</code> matches like <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">*.ads-example.com</code>)</li>
            <li>A trailing dot is stripped and ignored</li>
        </ul>
    );

    const ipAddressRules = (
        <div className="space-y-2">
            <p>When a domain resolves to many IP addresses, add one IP address associated with the domain, and access will be blocked.</p>
            <p>IP address ranges are not supported. Using a dash/hyphen/- between two IP addresses is not supported. CIDR notation is not supported.</p>
            <p>Wildcards (*) are not supported for IP addresses at the moment.</p>
        </div>
    );

    const howToCheckIfModDNSIsWorking = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>View the connection status displayed in the header of the modDNS web application</li>
            <li>Visit a known ad-heavy website and see if ads are blocked</li>
            <li>Check your Query Logs for recent activity</li>
            <li>Use <a href="https://www.dnsleaktest.com" target="_blank" rel="noopener noreferrer"><code className="text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">dnsleaktest.com</code></a> to verify you're using modDNS servers</li>
            <li>Try accessing a test domain that should be blocked</li>
        </ol>
    );

    const whyWebsitesAreNotWorking = (
        <div className="space-y-2">
            <p>This usually means a domain is being blocked by your enabled blocklists:</p>
            <ol className="list-decimal pl-5 space-y-1">
                <li>Check your Query Logs to see what's being blocked</li>
                <li>Add specific domains to your allowlist via Custom Rules</li>
                <li>Try disabling aggressive blocklists temporarily</li>
                <li>Contact support if issues persist</li>
            </ol>
        </div>
    );

    const whatIsDNSSEC = (
        <div className="space-y-2">
            <p>DNSSEC stands for Domain Name System Security Extensions. It's a security protocol that adds digital signatures to DNS records to ensure their authenticity and integrity. This helps prevent DNS spoofing attacks, where malicious actors could redirect users to fake websites.</p>
            <p>DNSSEC works by verifying that the DNS records have not been altered during transmission and that they come from a legitimate source. It's particularly important for securing domain names and ensuring that users reach the correct websites when they type URLs into their browsers.</p>
        </div>
    );

    const whatIsDNSSECDOBit = (
        <div className="space-y-2">
            <p>The DNSSEC OK (DO) bit, also known as the DNSSEC OK flag, is a flag in the DNS header that indicates whether the DNS client supports DNSSEC validation. When this bit is set to 1, it signals to the DNS resolver that the client wants the resolver to perform DNSSEC validation on the responses it provides.</p>
            <p>This allows the client to request signed DNS records and verify their authenticity. The DO bit was introduced to enable DNSSEC support in a backward-compatible manner, ensuring that clients and resolvers that do not support DNSSEC can still communicate with those that do.</p>
            <p>In modDNS, the DO bit is disabled by default to ensure compatibility with devices using systemd-resolved. DNSSEC validation is still performed automatically, even when the DO bit is not set.</p>
        </div>
    );

    const howToEnable2FA = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>Go to Account Settings</li>
            <li>Enable TOTP (Time-based One-Time Password)</li>
            <li>Scan the QR code with an authenticator app</li>
            <li>Enter the verification code to confirm</li>
            <li>Save your backup codes securely</li>
        </ol>
    );

    const sessionManagement = (
        <div className="space-y-2">
            <p>Logging into the account via the website creates a session. You can log out using any of these methods:</p>
            <ul className="list-disc pl-5 space-y-1">
                <li><strong>Navigation logout:</strong> Click the 'Log out' button in the left sidebar navigation menu</li>
                <li><strong>Header logout:</strong> When on the Account preferences page, click the 'Logout' button in the top header</li>
                <li><strong>Log out other sessions:</strong> In Account preferences, use 'Log out other web sessions' to log out all sessions except your current one - keeping you logged in while securing other devices</li>
            </ul>
            <p>The first two options will end your current session and redirect you to the login page. The third option keeps your current session active while logging out all other devices for enhanced security.</p>
        </div>
    );

    const howToDeleteAccount = (
        <ol className="list-decimal pl-5 space-y-1">
            <li>Click the account email address at the bottom of the page</li>
            <li>Click 'Account preferences'</li>
            <li>Click the 'Delete Account' button at the bottom of the page</li>
            <li>A confirmation window will appear. Entering an 8-symbol code is required for confirmation</li>
            <li>After entering the code, click the 'Delete Account' button. A 'Cancel' button is also available on this confirmation window</li>
        </ol>
    );

    const renderFAQContent = () => (
        <div className="space-y-6">
            <FAQSection title="Basics" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="What is modDNS?"
                    answer="modDNS is a privacy-focused DNS service that helps protect privacy and improve security by blocking ads, trackers, and malicious domains. It supports modern DNS protocols including DNS-over-HTTPS (DoH), DNS-over-TLS (DoT), and DNS-over-QUIC (DoQ)."
                />
                <FAQItem
                    question="How does modDNS protect my privacy?"
                    answer="By blocking known tracking domains, advertising networks, and malicious websites, using curated and custom blocklists, fewer data points about your online activities can be collected by privacy-invasive companies and information brokers. It also supports DNSSEC for additional security and provides detailed query logs (default: off) so you can monitor what's being blocked."
                />
                <FAQItem
                    question="Do you log my DNS queries?"
                    answer={
                        <p>Query logging is optional, off by default. When enabled, retention period is controlled by you, with logs available for review in your dashboard under the Query Logs tab. If query logs are turned off, we don't retain any information on your use of modDNS other than basic account information. Review our <span onClick={() => navigate('/privacy')} className="underline text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] cursor-pointer">Privacy Policy</span> for more information.</p>
                    }
                />
                <FAQItem
                    question="What DNS protocols do you support?"
                    answer={supportedProtocols}
                />
            </FAQSection>

            <FAQSection title="Account & Profiles" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="What are DNS profiles?"
                    answer="DNS profiles are custom configurations that determine which blocklists and settings apply to your setup. Each profile has a unique ID that you use to configure your devices."
                />
                <FAQItem
                    question="How do I create a DNS profile?"
                    answer={howToCreateProfile}
                />
                <FAQItem
                    question="How do I rename a DNS profile?"
                    answer={howToRenameProfile}
                />
                <FAQItem
                    question="How do I delete a DNS profile?"
                    answer={howToDeleteProfile}
                />
                <FAQItem
                    question="What are sessions and what happens if I choose to log out of them?"
                    answer={sessionManagement}
                />
                <FAQItem
                    question="How do I delete my account?"
                    answer={howToDeleteAccount}
                />
            </FAQSection>

            <FAQSection title="Device Identification" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="What is device identification and why would I use it?"
                    answer="Device identification allows you to distinguish between different devices using the same DNS profile. When enabled, you can see which specific device made each DNS query in your logs, making it easier to track and troubleshoot DNS issues across multiple devices."
                />
                <FAQItem
                    question="How do I configure device identification for DNS-over-HTTPS (DoH)?"
                    answer={
                        <div>
                            For DoH connections, add your device identifier to the URL path:
                            <br /><br />
                            <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">
                                https://dns.staging.ivpndns.net/dns-query/PROFILE_ID/DEVICE_ID
                            </code>
                            <br /><br />
                            Example: <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">https://dns.staging.ivpndns.net/dns-query/8kqr1tbfco/laptop</code>
                            <br /><br />
                            Replace PROFILE_ID with your actual profile ID and DEVICE_ID with a name for your device (e.g., "laptop", "phone", "router").
                        </div>
                    }
                />
                <FAQItem
                    question="How do I configure device identification for DNS-over-TLS (DoT) and DNS-over-QUIC (DoQ)?"
                    answer={
                        <div>
                            For DoT and DoQ connections, add your device identifier as a subdomain:
                            <br /><br />
                            <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">
                                DEVICE_ID--PROFILE_ID.dns.staging.ivpndns.net
                            </code>
                            <br /><br />
                            Examples:
                            <br />
                            • DoT (port 853): <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">laptop--8kqr1tbfco.dns.staging.ivpndns.net</code>
                            <br />
                            • DoQ (port 853): <code className="bg-[var(--shadcn-ui-app-muted)] text-[var(--shadcn-ui-app-foreground)] px-2 py-0.5 rounded text-sm font-mono border border-[var(--shadcn-ui-app-border)]">phone--8kqr1tbfco.dns.staging.ivpndns.net</code>
                            <br /><br />
                            Replace PROFILE_ID with your actual profile ID and DEVICE_ID with a name for your device.
                        </div>
                    }
                />
                <FAQItem
                    question="What are the rules for device identifiers?"
                    answer={
                        <div>
                            Device identifiers follow a simple, privacy-focused convention. The system automatically normalizes and truncates values when needed.
                            <br /><br />
                            <strong>Length & Truncation</strong>
                            <br />
                            • Maximum length: <strong>16 characters</strong> (default). Anything longer is silently truncated.
                            <br />
                            • Shorter names (1–16 chars) are used as-is.
                            <br /><br />
                            <strong>Characters</strong>
                            <br />
                            • Letters (a–z, A–Z), digits (0–9), spaces and hyphens are accepted in input.
                            <br />
                            • Apostrophes and other punctuation are stripped during normalization (e.g. Bob's iPhone → stored as <code>bobs iphone</code>). You may still include them in the DoH URL (e.g. <code>Bob%27s%20iPhone</code>) but they won't appear in logs.
                            <br />
                            • For DoT/DoQ, spaces are represented as <code>--</code> in the hostname ("Home Router" → <code>home--router</code>).
                            <br />
                            • Characters outside a–z, A–Z, 0–9, space, hyphen are removed for DoT/DoQ hostnames.
                            <br /><br />
                            <strong>Normalization</strong>
                            <br />
                            • Input is lowercased for DNS label usage where required.
                            <br />
                            • Multiple spaces are preserved for DoH but condensed by removal of disallowed characters for DoT/DoQ after substitution.
                            <br />
                            • Example truncation: <code>this-is-my-fantastic-work-laptop</code> → <code>this-is-my-fanta</code>
                            <br /><br />
                            <strong>Examples</strong>
                            <br />
                            Good: <code>laptop</code>, <code>home router</code>, <code>ipad-pro</code>, <code>Bob%27s%20iPhone</code> (appears in logs as <code>bobs iphone</code>)
                            <br />
                            Truncated: <code>verylongdevicename123</code> → <code>verylongdevicena</code>
                            <br />
                            Removed chars (DoT/DoQ): <code>my*game+pc</code> → <code>mygamepc</code>
                        </div>
                    }
                />
                <FAQItem
                    question="Can I see device identifiers even if IP logging is disabled?"
                    answer="Device identifiers are shown in query logs even when the 'Log clients IP' setting is disabled in your profile. This allows you to identify devices without logging IP addresses for enhanced privacy."
                />
            </FAQSection>

            <FAQSection title="Blocklists" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="What blocklists are available in modDNS?"
                    answer="We support a wide range of popular blocklists, including Hagezi, OISD, AdGuard, StevenBlack and others. We curate blocklists combinations for different needs (eg. Basic, Comprehensive, Restrictive) so you can get started with recommended lists easily."
                />
                <FAQItem
                    question="How do I enable/disable blocklists?"
                    answer={blocklistsInstructions}
                />
                <FAQItem
                    question="Can I block all DNS queries, instead of allowing all by default?"
                    answer={howToBlockAllQueries}
                />
                <FAQItem
                    question="How do I block subdomains in blocklists?"
                    answer="Subdomain blocking for blocklists is enabled by default. When enabled, if a blocklist contains a domain like example.com, all its subdomains (e.g. www.example.com) are also blocked. You can change this via 'Settings' > 'BLOCKLISTS' > 'Subdomains in blocklists'."
                />
                <FAQItem
                    question="Will blocklists break websites?"
                    answer="Some aggressive blocklists may occasionally block legitimate content. Start with 'Basic' protection and gradually add more comprehensive lists. You can always disable specific lists if you encounter issues."
                />
            </FAQSection>

            <FAQSection title="Custom Rules" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="How do I add a custom rule?"
                    answer={howToAddCustomRule}
                />
                <FAQItem
                    question="How are subdomains handled in custom rules?"
                    answer={customRulesSubdomainsInfo}
                />
                <FAQItem
                    question="What types of entries do Custom Rules support?"
                    answer={customRulesSupportedInputs}
                />
                <FAQItem
                    question="Which rules take precedence (allowlist, denylist, services, blocklists)?"
                    answer={rulePrecedenceInfo}
                />
                <FAQItem
                    question="Do I need to add a wildcard symbol (*) for a custom rule?"
                    answer={wildcardRules}
                />
                <FAQItem
                    question="How does a leading or trailing dot (.) affect a domain?"
                    answer={dotRules}
                />
                <FAQItem
                    question="When a domain resolves to many IP addresses, do I need to add them all?"
                    answer={ipAddressRules}
                />
            </FAQSection>

            <FAQSection title="Troubleshooting" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="How do I check if modDNS is working?"
                    answer={howToCheckIfModDNSIsWorking}
                />
                <FAQItem
                    question="Why are some websites not loading?"
                    answer={whyWebsitesAreNotWorking}
                />
            </FAQSection>

            <FAQSection title="Additional Settings" globalToggleSignal={toggleSignal} globalToggleState={toggleState}>
                <FAQItem
                    question="What is DNSSEC?"
                    answer={whatIsDNSSEC}
                />
                <FAQItem
                    question="What is the DNSSEC OK (DO) bit?"
                    answer={whatIsDNSSECDOBit}
                />
                <FAQItem
                    question="Do you support 2FA?"
                    answer={
                        <div>
                            Two-Factor Authentication adds an additional layer of security to your account. ModDNS supports 2FA, follow these steps:
                            <br />
                            {howToEnable2FA}
                        </div>
                    }
                />
            </FAQSection>
        </div>
    );

    return (
        <div className="relative min-h-screen w-full overflow-x-hidden bg-[var(--shadcn-ui-app-background)]">
            <div className="relative z-10 py-8">
                <div className="w-full max-w-4xl mx-auto p-8">
                    {hasHistory && (
                        <div className="mb-6">
                            <Button
                                onClick={() => navigate(-1)}
                                className="flex items-center gap-2 text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] bg-transparent hover:bg-transparent border-none p-0 font-inherit cursor-pointer"
                            >
                                <ArrowLeft className="h-4 w-4" />
                                Back
                            </Button>
                        </div>
                    )}

                    <Card className="bg-[var(--shadcn-ui-app-popover)] border-[var(--shadcn-ui-app-border)]">
                        <CardContent className="p-8">
                            <div className="flex flex-col items-center mb-8">
                                <img
                                    className="mb-4 w-[200px] h-10 mx-auto"
                                    alt="moddns logo"
                                    src={modDNSLogo}
                                />
                                <h1 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center font-mono">
                                    FAQ
                                </h1>
                            </div>

                            <div className="prose prose-invert max-w-none text-[var(--shadcn-ui-app-foreground)]">
                                <div className="mb-6">
                                    <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] mb-4">
                                        Last updated: {FAQ_LAST_UPDATED}
                                    </p>
                                    <div className="flex justify-end">
                                        <Button
                                            onClick={toggleAllFAQs}
                                            variant="outline"
                                            className="text-sm"
                                        >
                                            {toggleState ? 'Collapse All' : 'Expand All'}
                                        </Button>
                                    </div>
                                </div>

                                {renderFAQContent()}
                            </div>
                        </CardContent>
                    </Card>
                </div>
                <AuthFooter variant="relative" openInNewTab={false} />
            </div>
        </div>
    );
}
