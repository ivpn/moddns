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
                                            Last updated: 10 July 2025
                                        </p>
                                    </div>

                                    <section>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-6">
                                            modDNS is developed and operated by the team behind IVPN. All services we offer to customers are built for privacy. Internally, we know what exactly that means. If a choice needs to be made between one practice that deepens a user's privacy, and another that would diminish it but accelerate our growth, we'll always take the slower, more private option.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            We are committed to being transparent about how we collect, use, and protect your data after you sign up to the modDNS service. Below we offer a concise and human-readable summary of our policies.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What data don't you log?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            Our goal is minimal data collection. We don't log any customer related information unless it is absolutely required for the operation of the email forwarding service. As such, we don't log:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            <li>information about non-authenticated website visits</li>
                                            <li>we don't log customer IP addresses</li>
                                            <li>we don't log bandwidth usage</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Do you use cookies or third-party services?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            We use cookies solely for the purpose of managing authenticated sessions.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            No third-party cookies, trackers, or analytics are deployed when visiting our website or using the service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information do you store about my account?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            For users signing up with passkeys, we store the registered email address in our database.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            For users signing up with credentials, we store registered email address and a password hashed with Argon2 hashing algorithm.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            Further, we store additional email addresses optionally added as 'Recipient' for forwarded emails.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            We keep a record of the number of relayed or blocked emails for 90 days from the date and time the corresponding event occurred, used to provide relevant statistics to our customers.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">How do you process and what information you store about forwarded emails?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            Emails are processed through a self-hosted Postfix server.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            Delivered emails are immediately removed from the Postfix queues. Undelivered emails are stored in a deferred queue. After 5 days, Postfix stops attempting delivery and discards the emails from the deferred queue.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            The Postfix server is configured with the "info" log level, which means it records general information about email activity (such as when messages are sent or received). These logs are kept for 7 days to support system maintenance and troubleshooting.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">I've signed up to the service through my IVPN subscription. Can you associate my modDNS account with my IVPN Account ID?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            When your sign up through the IVPN service, a temporary modDNS signup link is generated in the IVPN database. Once the modDNS signup is completed, the link and corresponding identifiers are removed from the IVPN database to prevent any association between modDNS and IVPN accounts.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">What information is retained when I stop using your service?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            When a modDNS account is terminated due to subscription ending, non-payment or for any other reason, all settings and data associated with that modDNS account including the account itself is automatically deleted after 180 days. If you want to cancel your account and delete your data immediately, simply click on the 'Delete account' button in the Account settings.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Are there any ways to verify your claims regarding privacy?</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            The entire project is open source with repositories available on GitHub for customers to verify our privacy practices.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">User Rights</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            Reasonable requests for release of a specific user's data will be honoured within 28 days of an acceptable request from a user or person with a provable legal relationship with that user.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            We reserve the right to refuse or charge for requests that are manifestly unfounded or excessive. Any refused subject access requests will be responded to without undue delay including the refusal reason as well as recourse to refer to the supervisory authority.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Changes to policy</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            Privatus GmbH, operator of modDNS reserves the right to change this privacy policy at any time. In such cases, we will take every reasonable step to ensure that these changes are brought to your attention by posting all changes prominently on the modDNS website for a reasonable period of time, before the new policy becomes effective as well as emailing our existing customers.
                                        </p>
                                    </section>

                                    <div className="mt-8 pt-6 border-t border-[var(--shadcn-ui-app-border)]">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] text-center">
                                            If you have any questions or concerns about our privacy practices, contact us at <a href="mailto:moddns@ivpn.net" className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] transition-colors">moddns@ivpn.net</a>.
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
