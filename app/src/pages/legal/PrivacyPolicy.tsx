import { useNavigate, useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ArrowLeft } from "lucide-react";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg';
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg';
import { useTheme } from "@/components/theme-provider";
import AuthFooter from "@/components/auth/AuthFooter";

export default function PrivacyPolicy() {
    const navigate = useNavigate();
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark';
    const location = useLocation();
    const hasHistory = location.key !== "default";

    return (
        <div className="relative min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
            <div className="relative z-10 py-8">
                <div className="w-full max-w-4xl mx-auto p-8">
                    {hasHistory && (
                        <div className="mb-6">
                            <Button
                                variant="ghost"
                                onClick={() => navigate(-1)}
                                className="flex items-center gap-2 px-3 py-1.5 h-auto min-h-0 text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] hover:bg-black/5 dark:hover:bg-white/10 rounded-md"
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
                                    src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                />
                                <h1 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center font-mono">
                                    Privacy Policy
                                </h1>
                            </div>                            <div className="max-w-none text-[var(--shadcn-ui-app-foreground)]">
                                <div className="space-y-6">
                                    <div className="mb-6">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] mb-4">
                                            Last updated: Oct 28, 2026
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
                                            We use three cookies for signing in and setting up your account. All are set by modDNS itself and none are shared with third parties.
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
                                            <li>Your IP addresses</li>
                                            <li>Device information or identifiers</li>
                                            <li>Timestamps of DNS resolutions with user/profile attribution</li>
                                        </ul>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mt-4">
                                            For more information on what is stored when you optionally enable "Statistics" or "Query Logs", see the next section.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information is logged about DNS activity?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            ModDNS is a privacy-first service: logging of DNS related activity is disabled by default. Beyond this default setting you have complete control over Statistics and Query Logs for each profile.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">a) Default setting (Statistics and Query Logs Disabled)</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            When Statistics and Query Logs are turned off, no record of individual queries is written to disk.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            The only DNS activity data we keep is anonymous service-wide statistics: the number of queries, blocked queries and DNSSEC-validated queries handled by each server location, added up across all users with hourly timestamps. These counters contain no profile, device or client reference. Example of data stored:
                                        </p>
                                        <pre className="bg-[var(--shadcn-ui-app-background)] border border-[var(--shadcn-ui-app-border)] rounded-md p-4 text-sm text-[var(--shadcn-ui-app-foreground)] opacity-80 overflow-x-auto mb-4">{`"timestamp": "2026-09-16T10:00:00Z"
"pop": "ams1"
"queries": { "total": 18342, "blocked": 2917, "dnssec": 520 }`}</pre>

                                        <h3 className="text-lg font-semibold mb-2">b) With Statistics Enabled</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Statistics is an optional modDNS feature that counts the queries of a profile, so you can see how many were resolved and blocked without keeping a record of each query. It is off by default and you must explicitly turn it on for each profile.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We count queries per profile and device in 15-minute blocks. The data points counted include:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>The number of queries, blocked queries and DNSSEC-validated queries</li>
                                            <li>The type of rule that blocked a query (blocklist, service, custom rule, DNS rebinding protection or default rule), without naming the blocklist, service or rule</li>
                                            <li>The protocol used (DoH, DoT or DoQ)</li>
                                            <li>Your Device Identifier, if configured</li>
                                        </ul>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mt-4 mb-4">
                                            Statistics contain no domains, no IP addresses and no time of individual queries. We also store the time you turned Statistics on. Example of data stored:
                                        </p>
                                        <pre className="bg-[var(--shadcn-ui-app-background)] border border-[var(--shadcn-ui-app-border)] rounded-md p-4 text-sm text-[var(--shadcn-ui-app-foreground)] opacity-80 overflow-x-auto mb-4">{`"bucket_start": "2026-10-23T10:15:00Z"
"meta": { "profile_id": "3mdq3557b1", "device_id": "ff_browser" }
"total": 412, "blocked": 57, "dnssec": 38
"reason_blocklist": 49, "reason_service": 0, "reason_custom_rule": 6,
"reason_rebinding": 0, "reason_default_rule": 2, "reason_other": 0
"proto_doh": 301, "proto_dot": 111, "proto_doq": 0`}</pre>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Counts become less detailed as they age: 15-minute counts are kept for 1 day and hourly counts for 8 days. Daily totals are kept for the retention period you choose: 30 days by default, 90 days or 1 year. Counts are deleted automatically once they exceed these periods. Deletion process may take up to 24 hours.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            When you shorten the retention period, daily totals older than the new period are deleted. Turning Statistics off, or using Delete statistics history, permanently deletes all statistics for the profile.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">c) With Query Logs Enabled</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Query Logs is a modDNS feature that provides a detailed log of all queries to help you monitor and fine-tune your DNS filtering. If you choose to enable query logging, we store data required to provide you with detailed reports on your DNS activity. This is an optional feature that you must explicitly opt in to. You also have detailed control over what is logged and for how long.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            The data points logged include:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>The timestamp of the query</li>
                                            <li>The requested domain or IP address (e.g., example.com), if enabled</li>
                                            <li>The decision (e.g., Allowed, Blocked) and the trigger for the decision (e.g. entry on enabled blocklist)</li>
                                            <li>Your client IP address, if enabled (off by default)</li>
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
                                            Within your profile settings, you can independently enable or disable the logging of queried domains and client IP addresses.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You also have full control over the data retention period, which can be set from a minimum of 1 hour to a maximum of 1 month. The default retention period is 1 hour.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Query log entries are automatically and permanently deleted once they exceed the retention period, which can take up to an hour. When you change the retention period setting, any existing logs that exceed the new timeframe are deleted from our systems. You can permanently delete all stored query logs at any time from your dashboard, and turning Query Logs off deletes them as well.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            When Query Logs and client IP logging are both on, the Top clients list on the Statistics page shows the network provider and country of each logged address. We look these up in an IP-geolocation database held on our own servers each time you open the page. Nothing is sent to a third party, and the result is not stored. The addresses themselves are part of your query logs and are kept for the retention period you have set.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">How do you handle Device Identification?</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            Device Identification is an optional feature that allows you to distinguish between devices using the same DNS profile. The device identifier you set is only stored if you have Statistics or Query Logs enabled.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            This feature can be used to identify devices in your statistics and logs without needing to log your IP address, offering an additional layer of privacy when using Query Logs.
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
                                            If you choose to delete your account via the 'Delete Account' button in your Account settings, all data associated with your account - including your email, profile configurations, statistics and any stored query logs - is deleted immediately.
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
                                            <strong>If Statistics and Query Logs are disabled:</strong> We hold no DNS activity data that can be linked to you. We can only be compelled to provide the account and profile information described in "What information do you store about my account?"
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>If Statistics is enabled:</strong> We have the query counts per device that you have chosen to keep, for the retention period you have set. They contain no domains or IP addresses.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <strong>If Query Logging is enabled:</strong> We have query logs that you have chosen to store. The extent of this data depends entirely on the logging and retention settings you have configured.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            If this concerns you, we recommend keeping Statistics and Query Logs disabled. We cannot be compelled to hand over data we do not hold.
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
