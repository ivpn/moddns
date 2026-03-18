import { useNavigate, useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ArrowLeft } from "lucide-react";
import modDNSLogo from '@/assets/logos/modDNS.svg';
import AuthFooter from "@/components/auth/AuthFooter";

export default function PrivacyPolicy() {
    const navigate = useNavigate();
    const location = useLocation();
    const hasHistory = location.key !== "default";

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
                                    alt="modDNS logo"
                                    src={modDNSLogo}
                                />
                                <h1 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center font-mono">
                                    Privacy Policy
                                </h1>
                            </div>                            <div className="prose prose-invert max-w-none text-[var(--shadcn-ui-app-foreground)]">
                                <div className="space-y-6">
                                    <div className="mb-6">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] mb-4">
                                            Last updated: Mar 16, 2026
                                        </p>
                                    </div>

                                    <section>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-6">
                                            modDNS is developed and operated by IVPN Limited. All services we offer are built for privacy. If a choice needs to be made between a practice that deepens a user's privacy and one that would reduce it but accelerate our growth, we will always take the slower, more private option.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            Below is a concise and human-readable privacy policy overview for the modDNS service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information do you store about my account?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            To operate your modDNS account, we store the following information:
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>Authentication Credentials:</strong> Your registered email address and a hashed password. If you use passkeys, we store the necessary identifiers to authenticate you. If you enable Two-Factor Authentication (2FA), we also store your backup codes for account recovery.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>Profile Configuration:</strong> All settings associated with your DNS profiles, such as enabled blocklists, custom rules, and other preferences.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>Active Sessions:</strong> We maintain a record of your active web sessions to improve account security and provide session management features. We do not store IP addresses, device information or other personal identifiers related to sessions. Example of what is stored per session:
                                        </p>
                                        <pre className="bg-[var(--shadcn-ui-app-background)] border border-[var(--shadcn-ui-app-border)] rounded-md p-4 text-sm text-[var(--shadcn-ui-app-foreground)] opacity-80 overflow-x-auto mb-4">{`{
  "_id": {
    "$oid": "68f73b254230d85ad36e4424"
  },
  "token": "JBj21xnnAjF4GS9u+JaE9Ux+KDRa1EBJd9qW+Btl/Cg=",
  "account_id": "687a8d6280d5bcea67bb04b9",
  "data": {
    "$binary": {
      "base64": "eyJjaGFsbGVuZ2UiOiAiLQJycAlkAjoiQiwidbNlcl8pZCI5Ik4qZzNZVShrQmpYAE2HUTNaR05rWmpZNVltSXdNMkU1IiwiZXhwaXJlcyI6IjIwMjUtMTAtMjFUMTU6NDk6NTcuMDM2ODI5M1oiLCJ1c2VyVmVyaWZpY2F0aW9uIjoiIn0=",
      "subType": "00"
    }
  },
  "last_modified": {
    "$date": "2025-10-21T07:49:57.036Z"
  }
}`}</pre>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Do you use cookies or third-party services?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We use a single cookie solely for the purpose of managing your authenticated session when you are logged into the modDNS website.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            For monitoring aggregate website usage we run our own <a href="https://umami.is/docs" target="_blank" rel="noopener noreferrer" className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] transition-colors">Umami</a> analytics instance. Umami is a privacy-focused, cookieless solution that tracks page views and user interactions without collecting personally identifiable information. All data remains on our self-hosted infrastructure.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What data don't you log?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Our goal is zero or minimal data collection. We do not log any customer-related information unless it is absolutely required for the operation of the service. By default, we do not log:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>DNS queries (e.g., which websites you visit)</li>
                                            <li>Timestamps of DNS resolutions</li>
                                            <li>Your IP addresses</li>
                                            <li>Device information or identifiers</li>
                                        </ul>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mt-4">
                                            For more information on what is logged when you optionally enable "Query Logs", see the next section.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information is logged about DNS activity?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            ModDNS is a privacy-first service: logging of DNS related activity is disabled by default. Beyond this default setting you have complete control over your DNS query logging.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">a) Default setting (Query Logs Disabled)</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            When query logging is turned off, all queries are processed entirely in memory and are never written to disk. We log no information about your usage of the DNS resolver, with one exception:
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We store a total count of DNS requests processed by your profile. This is a simple counter and contains no specific details about your activity. Example of data stored:
                                        </p>
                                        <pre className="bg-[var(--shadcn-ui-app-background)] border border-[var(--shadcn-ui-app-border)] rounded-md p-4 text-sm text-[var(--shadcn-ui-app-foreground)] opacity-80 overflow-x-auto mb-4">{`"profile_id": "ju8eamnqfn"
"queries": { "total": 244 }`}</pre>

                                        <h3 className="text-lg font-semibold mb-2">b) With Query Logs Enabled</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Query Logs is a modDNS feature that provides a detailed log of all queries to help you monitor and fine-tune your DNS filtering. If you choose to enable query logging, we store data required to provide you with detailed reports on your DNS activity. This is an optional feature that you must explicitly opt-in to, further, you have detailed control over what is logged and for how long.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            The data points logged include:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>The timestamp of the query</li>
                                            <li>The requested domain or IP address (e.g., example.com)</li>
                                            <li>The decision (e.g., Allowed, Blocked) and the trigger for the decision (e.g. entry on enabled blocklist)</li>
                                            <li>Your client IP address</li>
                                            <li>Your Device Identifier, if configured</li>
                                        </ul>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mt-4 mb-4">
                                            Example of data stored for a query with all Query Log options enabled and configured:
                                        </p>
                                        <pre className="bg-[var(--shadcn-ui-app-background)] border border-[var(--shadcn-ui-app-border)] rounded-md p-4 text-sm text-[var(--shadcn-ui-app-foreground)] opacity-80 overflow-x-auto mb-4">{`"timestamp": "$date": "2025-09-17T08:15:45.813Z"
"profile_id": "3mdq3557b1",
"status": "blocked",
"device_id": "ff_browser",
"reasons": "blocklist: hagezi_multi_ultimate"
"protocol": "https",
"dns_request":
    "domain": "ads.example.org",
    "query_type": "AAAA",
    "response_code": "NOERROR",
    "dnssec": false
"client_ip": "203.0.113.55"`}</pre>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Within your profile settings, you can independently enable or disable the logging of queried domains and client IP Addresses.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You also have full control over the data retention period, which can be set from a minimum of 1 hour to a maximum of 1 month. The default retention period is 1 hour. When you change the retention period setting, any existing logs that exceed the new timeframe are immediately deleted from our systems.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            You can permanently delete all stored query logs at any time from your dashboard.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">How do you handle Device Identification?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Device Identification is an optional feature that allows you to distinguish between devices using the same DNS profile. The device identifier you set is only logged if you have Query Logs enabled.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            This feature can be used to identify devices in your logs without needing to log your IP address, offering an additional layer of privacy when using Query Logs.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Can I verify your privacy claims?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            The entire modDNS project is open source. All code is available in public repositories on GitHub for customers to inspect and verify our privacy practices and the implementation of this policy.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information is retained when I stop using your service?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            If you choose to delete your account via the 'Delete Account' button in your Account settings, all data associated with your account - including your email, profile configurations, and any stored query logs - is deleted immediately.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            Once an account is deleted, no data associated with it can be recovered.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">How do you respond to legal requests for data?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            IVPN Limited is registered in Gibraltar. We only respond to requests from recognized government and law enforcement agencies with jurisdiction in Gibraltar. We do not respond to requests from agencies outside of Gibraltar.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            If we receive a binding court order, we can only provide the data we possess.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>If Query Logging is disabled:</strong> We have no DNS activity logs to share. We can only be compelled to confirm that an account exists for a specific email address.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <strong>If Query Logging is enabled:</strong> We have query logs that you have chosen to store. The extent of this data depends entirely on the logging and retention settings you have configured. If you are concerned about this possibility, we recommend keeping Query Logs disabled or regularly purging your logs.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">User Rights</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            In accordance with GDPR, reasonable requests for access to your personal data will be honored within 28 days.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            We reserve the right to refuse or charge for requests that are clearly unreasonable or excessive. Any refused request will be met with a timely response explaining the reason for refusal and your right to refer the matter to our supervisory authority, the Gibraltar Regulatory Authority.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Changes to policy</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            IVPN Limited reserves the right to change this privacy policy at any time. In such cases, we will take every reasonable step to ensure these changes are brought to your attention.
                                        </p>
                                    </section>

                                    <div className="mt-8 pt-6 border-t border-[var(--shadcn-ui-app-border)]">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] text-center">
                                            If you have any questions or concerns about our privacy practices, please contact us at <a href="mailto:moddns@ivpn.net" className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] transition-colors">moddns@ivpn.net</a>.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </div>

                <AuthFooter variant="relative" openInNewTab={false} />
            </div>
        </div>
    );
}
