import { useNavigate, useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ArrowLeft } from "lucide-react";
import modDNSLogo from '@/assets/logos/modDNS.svg';
import AuthFooter from "@/components/auth/AuthFooter";

export default function TermsOfService() {
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
                                    Terms of Service
                                </h1>
                            </div>

                            <div className="prose prose-invert max-w-none text-[var(--shadcn-ui-app-foreground)]">
                                <div className="space-y-6">
                                    <div className="mb-6">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] mb-4">
                                            Last updated: 23 April 2025
                                        </p>
                                    </div>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">1. Introduction</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            These Terms of Service ("Terms") govern your use of modDNS, a DNS resolver and filtering service operated by IVPN Limited ("we," "us," or "our"). By subscribing to or using our service, you ("you" or "user") agree to abide by these Terms.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">2. Acceptance of Terms</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            By accessing or using our service, you agree to be bound by these Terms and our{" "}
                                            <span
                                                onClick={() => navigate('/privacy')}
                                                className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] cursor-pointer"
                                            >
                                                Privacy Policy
                                            </span>, which is incorporated by reference. If you do not agree with any part of these Terms, do not use our service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">3. User Responsibilities</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            You agree to use our service only for lawful purposes and in a manner that does not infringe or restrict any third party's rights or service use. Prohibited activities include, but are not limited to:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            <li>Engaging in illegal activities, including criminal or terrorist actions, fraud, identity theft, and money laundering.</li>
                                            <li>Attempting to hack, attack, or gain unauthorized access to any computers, networks, accounts, or systems.</li>
                                            <li>Transmitting or distributing viruses, worms, trojans, or other malicious software.</li>
                                            <li>Sending harassing, defamatory, abusive, threatening, or hateful messages.</li>
                                            <li>Sending unsolicited emails, bulk messages, or any form of spam, including commercial advertising.</li>
                                            <li>Engaging in phishing or activities that attempt to steal personal information.</li>
                                            <li>Distributing or transmitting copyrighted materials without proper authorization.</li>
                                            <li>Participating in activities related to child pornography or exploitation.</li>
                                            <li>Using stolen or unauthorized payment information, including credit card numbers and online payment accounts.</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">4. Account Usage and Security</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            Your account is for individual use only; sharing your account or login credentials is prohibited. You are responsible for all activities that occur under your account. Notify us immediately of any unauthorized use or security breaches.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">5. Payment Terms</h2>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            <li>All payments must be made using valid and authorized payment methods. Do not use stolen or unauthorized payment information.</li>
                                            <li>Fees are due in advance and are non-refundable, except as required by law or specified in our Refund Policy.</li>
                                            <li>We reserve the right to change our fees at any time. Advance notice of fee changes will be provided by posting updates on our website and social media communication channels.</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">6. Service Availability and Modifications</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            We strive to provide reliable service but do not guarantee uninterrupted availability. We reserve the right to modify, suspend, or discontinue the service (or any part thereof) at any time, with or without notice.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            Technical support is provided via email and is limited to issues related to the functionality of the service.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">7. Termination and Suspension</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            We may suspend or terminate your account immediately, without prior notice or liability, upon breach of these Terms. No refunds will be provided if your account is terminated due to a breach of these Terms.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            Upon termination, your right to use the service ceases immediately. Provisions that should survive termination will remain in effect, including ownership provisions, warranty disclaimers, indemnity, and limitations of liability.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">8. Changes to Terms</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            We reserve the right to modify or replace these Terms at any time. Significant changes will be communicated by posting the new Terms on our website and updating the "Effective Date." By continuing to use our service after revisions become effective, you agree to be bound by the revised Terms.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">9. Limitation of Liability and Disclaimer of Warranties</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            The service is provided on an "as is" and "as available" basis, without any warranties or conditions, express or implied. We do not warrant that the service will meet your requirements or that it will be uninterrupted, timely, or error-free.
                                        </p>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            In no event shall we be liable for any indirect, incidental, special, consequential, or punitive damages, including loss of profits, data, use, goodwill, or other intangible losses.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">10. Indemnification</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed mb-4">
                                            You agree to defend, indemnify, and hold harmless IVPN Limited and its officers, directors, employees, and agents from any claims, damages, losses, liabilities, costs, or expenses arising from:
                                        </p>
                                        <ul className="list-disc pl-6 space-y-2 text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            <li>Your use of and access to the service.</li>
                                            <li>Your violation of any term of these Terms.</li>
                                            <li>Your violation of any third-party right, including intellectual property or privacy rights.</li>
                                        </ul>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">11. Governing Law</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            These Terms are governed by and construed in accordance with the laws of Gibraltar, without regard to its conflicts of law provisions.
                                        </p>
                                    </section>

                                    <section>
                                        <h2 className="text-xl font-semibold mb-3">Contact Us</h2>
                                        <p className="text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                            If you have any questions about these Terms, please contact us at <a href="mailto:moddns@ivpn.net" className="text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] transition-colors">moddns@ivpn.net</a>.
                                        </p>
                                    </section>

                                    <div className="mt-8 pt-6 border-t border-[var(--shadcn-ui-app-border)]">
                                        <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] text-center">
                                            This document constitutes the entire Terms and Conditions for your use of the modDNS, a DNS resolver and filtering service operated by IVPN Limited.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </div>

                <AuthFooter openInNewTab={false} />
            </div>
        </div>
    );
}
