import { useNavigate, useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ArrowLeft } from "lucide-react";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg';
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg';
import { useTheme } from "@/components/theme-provider";
import AuthFooter from "@/components/auth/AuthFooter";

export default function TermsOfService() {
    const navigate = useNavigate();
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    const location = useLocation();
    const hasHistory = location.key !== "default";

    return (
        <div className="relative min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
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
                                    src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                />
                                <h1 className="text-2xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center font-mono">
                                    Terms of Service
                                </h1>
                            </div>

                            <div className="prose prose-invert max-w-none text-[var(--shadcn-ui-app-foreground)]">
                                <div className="space-y-6">
                                    <div className="mb-6">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] mb-4">
                                            Last updated: Mar 15, 2026
                                        </p>
                                    </div>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">1. Introduction</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            These Terms of Service (&ldquo;Terms&rdquo;) outline your use of modDNS, operated by IVPN Limited (&ldquo;we,&rdquo; &ldquo;us,&rdquo; or &ldquo;our&rdquo;). By using our service, you (&ldquo;you&rdquo; or &ldquo;user&rdquo;) agree to these Terms.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">2. Definition of Service</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            modDNS (the &ldquo;Service&rdquo;) is a DNS filtering service that resolves domain name queries based on a user-defined set of rules. The primary function of the Service is to either permit or block DNS resolutions based on the user&rsquo;s selected blocklists and custom filtering rules.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">3. Acceptance of Terms</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            By accessing or using our service, you agree to these Terms and our{" "}
                                            <span
                                                onClick={() => navigate('/privacy')}
                                                className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] cursor-pointer"
                                            >
                                                Privacy Policy
                                            </span>. If you do not agree with any part of these Terms, do not use our service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">4. User Responsibilities and Prohibited Uses</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You agree to use our service only for lawful purposes. You agree not to use the Service for any of the following prohibited activities:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            <li>Illegal activities such as criminal or terrorist actions, fraud, identity theft, and money laundering</li>
                                            <li>Hacking, attacking, or gaining unauthorized access to computers, networks, accounts, or systems</li>
                                            <li>Distributing malicious software including viruses, worms, or trojans</li>
                                            <li>Harassment, defamation, abuse, threats, or hate activities</li>
                                            <li>Activities related to child pornography or exploitation</li>
                                            <li>Phishing or identity theft schemes</li>
                                            <li>Distributing copyrighted materials without permission</li>
                                        </ul>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You also agree that you will not:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>Attack, overwhelm, or attempt to disrupt our DNS servers through denial-of-service attacks or similar means</li>
                                            <li>Generate excessive query loads through bots, scripts, or other automated means</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">5. Account Usage and Security</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            Your account is for individual use only. You are responsible for all activities that happen under your account, including all DNS queries made through your configured profiles. Notify us immediately of any unauthorized use or security breaches.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">6. Responsibilities and Disclaimers</h2>

                                        <h3 className="text-lg font-semibold mb-2">Profile Configuration</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You have full control over your DNS profile configuration, including custom allow/deny rules and blocklist selections. You are fully responsible for the consequences of any blocklist settings or custom rules you configure, including any domains you choose to block or allow.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">Blocklist Accuracy</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We are not responsible for the accuracy of third-party blocklists and are not liable for any damages resulting from a domain being incorrectly blocked or allowed. Blocklists are maintained by third parties and may contain errors.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">Security Limitations</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            The modDNS service is a DNS filtering tool that helps block known threats, but it is not a complete security solution. It does not guarantee website access, internet connectivity, or complete protection against malware, phishing, and other online threats. You may still encounter malicious content through other means or threats not covered by blocklists offered through our service.
                                        </p>

                                        <h3 className="text-lg font-semibold mb-2">Technical Limitations</h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            We do not guarantee the accuracy of DNS resolution, continuous availability of all DNS protocols (DoH, DoT, DoQ), or uninterrupted service. DNS queries may occasionally fail or return unexpected results due to technical limitations beyond our control.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">7. Intellectual Property &amp; License</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            IVPN Limited owns all rights to the modDNS interface, graphics, and logos, except where covered by open source licenses.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We grant you a personal, non-exclusive, non-transferable, revocable license to use the service, in accordance with these Terms.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            The modDNS project is open source under the GPL-3.0 license. You may review, modify, and distribute the source code available on GitHub in accordance with the applicable open source license terms.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">8. Service Availability and Termination</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We try to provide reliable service but do not guarantee uninterrupted availability. We reserve the right to modify, suspend, or stop the service at any time, with or without notice.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            We may implement rate limiting or temporary suspension of accounts that generate unreasonable and excessive query loads that impact service performance for other users.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            We may suspend or terminate your account immediately, without prior notice, if you breach these Terms. When your account is terminated, you lose access to the service immediately. All your account data will be permanently deleted and cannot be recovered.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">9. Limitation of Liability</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            The service is provided on an &ldquo;as is&rdquo; and &ldquo;as available&rdquo; basis. To the fullest extent permitted by law, we make no express or implied warranties of any kind. This means we don&rsquo;t promise it will meet your specific needs or work perfectly.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            We are not liable for any direct, indirect, incidental, special, consequential, or punitive damages, including loss of profits, data, or other intangible losses caused by your use of the service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">10. Indemnification</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            You agree to defend, indemnify, and hold harmless IVPN Limited and its officers, directors, employees, and agents from any claims, damages, losses, or expenses related to:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            <li>Your use of the service</li>
                                            <li>Your violation of these Terms</li>
                                            <li>Your violation of any third-party rights</li>
                                            <li>Your custom DNS configuration choices</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">11. Changes to Terms</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            We reserve the right to modify these Terms at any time. Significant changes will be communicated by posting the new Terms on our website. By continuing to use our service after the changes take effect, you agree to the new Terms.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">12. Governing Law</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed mb-4">
                                            These Terms are governed by and construed in accordance with the laws of Gibraltar.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            You agree that any legal dispute related to these Terms or the Service will be handled exclusively by the courts of Gibraltar.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Contact Us</h2>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] leading-relaxed">
                                            If you have any questions about these Terms, please contact us at <a href="mailto:moddns@ivpn.net" className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] transition-colors">moddns@ivpn.net</a>.
                                        </p>
                                    </section>

                                    <div className="mt-8 pt-6 border-t border-[var(--shadcn-ui-app-border)]">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] text-center">
                                            IVPN Limited<br />Incorporated in Gibraltar
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
