import { Label as ShadcnLabel } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import React, { type JSX, useState } from "react";
import AuthFooter from "@/components/auth/AuthFooter";
import { Input } from "@/components/ui/input";
import { ArrowLeft, Mail } from "lucide-react";
import { useNavigate } from "react-router-dom";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg';
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg';
import { useTheme } from "@/components/theme-provider";
import api from "@/api/api";
import { toast } from "sonner";
import type { RequestsResetPasswordBody } from "@/api/client";

export default function PasswordReset(): JSX.Element {
    const navigate = useNavigate();
    const [email, setEmail] = useState("");
    const [loading, setLoading] = useState(false);
    const [sent, setSent] = useState(false);
    const [resending, setResending] = useState(false);
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);

    const handleReset = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        try {
            const body = {
                email: email,
            } as RequestsResetPasswordBody;
            const response = await api.Client.accountsApi.apiV1AccountsResetPasswordPost(body);
            if (response.status === 204) {
                setSent(true);
                toast.success("Reset instructions sent to your email.");
            }
        } catch (err: unknown) {
            const axiosErr = err as { response?: { data?: { error?: string } } };
            toast.error(axiosErr?.response?.data?.error || "Failed to send reset instructions.");
        } finally {
            setLoading(false);
        }
    };

    const handleResend = async () => {
        setResending(true);
        try {
            const body = {
                email: email,
            } as RequestsResetPasswordBody;
            const response = await api.Client.accountsApi.apiV1AccountsResetPasswordPost(body);
            if (response.status === 204) {
                toast.success("Reset instructions resent.");
            }
        } catch (err: unknown) {
            const axiosErr = err as { response?: { data?: { error?: string } } };
            toast.error(axiosErr?.response?.data?.error || "Failed to resend reset instructions.");
        } finally {
            setResending(false);
        }
    };

    return (
        <div className="relative flex flex-col min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
            {/* Main content area - centered vertically and horizontally */}
            <div className="flex-1 flex items-center justify-center safe-px py-8">
                <div className="flex flex-col auth-shell items-center gap-4 px-4 sm:px-0">
                    <Card className="w-full bg-[var(--shadcn-ui-app-popover)] border-[var(--shadcn-ui-app-border)] rounded-[var(--primitives-radius-radius-md)]">
                        <CardContent className="flex flex-col items-center gap-8 p-11">
                            {/* Logo */}
                            <div className="flex items-center justify-center mb-4">
                                <img
                                    className="w-[200px] h-10"
                                    alt="modDNS logo"
                                    src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                    style={{ display: "block" }}
                                />
                            </div>

                            <div className="flex flex-col items-center gap-8 w-full">
                                {!sent ? (
                                    <>
                                        <div className="flex flex-col items-center gap-4">
                                            <div className="w-[190px] h-4 flex items-center justify-center">
                                                <ShadcnLabel className="font-mono font-bold text-[var(--shadcn-ui-app-foreground)] text-lg text-center tracking-[-0.60px] leading-4 whitespace-nowrap">
                                                    Forgot your password?
                                                </ShadcnLabel>
                                            </div>

                                            <div className="flex flex-col items-center gap-2">
                                                <p className="font-normal text-[var(--shadcn-ui-app-secondary-foreground)] text-sm text-center tracking-[0] leading-[14px]">
                                                    No worries, we&apos;ll send you reset instructions
                                                </p>
                                            </div>
                                        </div>

                                        <form className="flex flex-col items-start gap-8 w-full" onSubmit={handleReset}>
                                            <div className="flex flex-col items-start gap-4 w-full">
                                                <div className="relative w-full">
                                                    <Mail className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[var(--tailwind-colors-slate-400)]" />
                                                    <Input
                                                        type="email"
                                                        placeholder="Email address"
                                                        className="pl-10 bg-[var(--shadcn-ui-app-muted)] focus:bg-[var(--tailwind-colors-slate-700)] border border-[var(--shadcn-ui-app-border)] text-[var(--shadcn-ui-app-foreground)] placeholder:text-[var(--tailwind-colors-slate-400)] font-normal text-sm rounded-md"
                                                        value={email}
                                                        onChange={e => setEmail(e.target.value)}
                                                        required
                                                        disabled={loading}
                                                    />
                                                </div>
                                            </div>

                                            <div className="flex flex-col items-start gap-4 w-full">
                                                <Button
                                                    type="submit"
                                                    className="w-full bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)]/90 text-[var(--shadcn-ui-app-background)] rounded-[var(--primitives-radius-radius-md)] h-auto py-2 px-3 font-medium text-sm"
                                                    disabled={loading}
                                                >
                                                    {loading ? "Sending..." : "Send reset link"}
                                                </Button>

                                                <Button
                                                    variant="secondary"
                                                    className="w-full bg-[var(--tailwind-colors-slate-800)] hover:bg-[var(--tailwind-colors-slate-900))]/90 text-[var(--tailwind-colors-rdns-600)] rounded-[var(--primitives-radius-radius-md)] h-auto py-1.5 px-2 font-medium text-sm"
                                                    onClick={() => navigate("/login")}
                                                    type="button"
                                                >
                                                    <ArrowLeft className="w-4 h-4 mr-2" />
                                                    Back to log in
                                                </Button>
                                            </div>
                                        </form>
                                    </>
                                ) : (
                                    <>
                                        <div className="flex flex-col items-center gap-11 w-full">
                                            <div className="inline-flex flex-col items-center gap-4">
                                                <div className="w-[190px] h-4 flex items-center justify-center">
                                                    <ShadcnLabel className="font-mono font-bold text-[var(--shadcn-ui-app-foreground)] text-xl text-center tracking-[-0.60px] leading-4 whitespace-nowrap">
                                                        Check your email
                                                    </ShadcnLabel>
                                                </div>


                                                <div className="inline-flex flex-col items-center gap-2">
                                                    <div className="font-normal text-[var(--shadcn-ui-app-secondary-foreground)] text-sm text-center tracking-[0] leading-[14px]">
                                                        <span className="text-[var(--shadcn-ui-app-secondary-foreground)]">
                                                            We sent a password reset link to
                                                            <br />
                                                        </span>
                                                        <span className="font-semibold text-[var(--tailwind-colors-rdns-600)] leading-[25.2px]">
                                                            {email}
                                                        </span>
                                                    </div>
                                                </div>
                                            </div>

                                            <div className="flex flex-col items-start gap-8 w-full">
                                                <div className="flex flex-col items-start gap-4 w-full">
                                                    <Button
                                                        className="w-full h-auto bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)]/90 text-[var(--shadcn-ui-app-background)] rounded-[var(--primitives-radius-radius-md)] py-2 px-3 font-medium text-sm"
                                                        onClick={() => navigate("/login")}
                                                    >
                                                        Back to log in
                                                    </Button>

                                                    <div className="flex items-center justify-center gap-[5px] w-full">
                                                        <span className="font-normal text-[var(--shadcn-ui-app-muted-foreground)] text-xs text-center tracking-[0] leading-3 whitespace-nowrap">
                                                            Didn&apos;t receive the email?
                                                        </span>
                                                        <button
                                                            className="inline-flex items-center justify-center"
                                                            onClick={handleResend}
                                                            disabled={resending}
                                                        >
                                                            <span className="font-medium text-[var(--shadcn-ui-app-foreground)] text-xs hover:underline cursor-pointer">
                                                                {resending ? "Resending..." : "Click to resend"}
                                                            </span>
                                                        </button>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </>
                                )}
                            </div>
                        </CardContent>
                    </Card>
                </div>
            </div>

            {/* AuthFooter pinned to bottom with proper spacing */}
            <div className="w-full px-4 pb-8 pt-16">
                <AuthFooter />
            </div>
        </div>
    );
}
