import { Label as ShadcnLabel } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ArrowLeft } from "lucide-react";
import React, { useState, type JSX } from "react";
import modDNSLogoDarkTheme from "@/assets/logos/modDNS-dark-theme.svg";
import modDNSLogoLightTheme from "@/assets/logos/modDNS-light-theme.svg";
import { useTheme } from "@/components/theme-provider";
import AuthFooter from "@/components/auth/AuthFooter";
import api from "@/api/api";
import { toast } from "sonner";
import { useNavigate, useParams } from "react-router-dom";
import { PASSWORD_COMPLEXITY_RULES } from '@/lib/consts';

export default function PasswordResetConfirm(): JSX.Element {
    const navigate = useNavigate();
    const { token } = useParams();
    const [newPassword, setNewPassword] = useState("");
    const [confirmPassword, setConfirmPassword] = useState("");
    const [otp, setOtp] = useState("");
    const [showOtp, setShowOtp] = useState(false);
    const [loading, setLoading] = useState(false);
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);

    const handleReset = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!token) {
            toast.error("Invalid or missing token.");
            return;
        }
        if (!newPassword || !confirmPassword) {
            toast.error("Please fill in all fields.");
            return;
        }
        if (newPassword !== confirmPassword) {
            toast.error("Passwords do not match.");
            return;
        }
        setLoading(true);
        try {
            const data = {
                token,
                new_password: newPassword,
            };
            const mfaMethods = ["totp"];
            await api.Client.verificationApi.apiV1VerifyResetPasswordPost(data, otp, mfaMethods);
            navigate("/login", {
                state: { passwordResetSuccess: true }
            });
        } catch (err: unknown) {
            const axiosErr = err as { response?: { status?: number; data?: { error?: string; details?: string | string[] } } };
            switch (axiosErr?.response?.status) {
                case 400: {
                    // Handle both string and array in details
                    const errorMsg = axiosErr?.response?.data?.error;
                    const details = axiosErr?.response?.data?.details;
                    switch (errorMsg) {
                        case "token expired":
                            toast.error("Reset password token has expired. Please request a new one.");
                            break;
                        case "invalid request":
                            if (
                                (typeof details === "string" && details.includes("password")) ||
                                (Array.isArray(details) && details.some((d: string) => d.toLowerCase().includes("password")))
                            ) {
                                toast.error(PASSWORD_COMPLEXITY_RULES);
                            } else {
                                toast.error("Invalid request. Please check your input.");
                            }
                            break;
                        default:
                            toast.error(errorMsg || "Invalid request. Please check your input.");
                            break;
                    }
                    break;
                }
                case 401:
                    if (axiosErr?.response?.data?.error === "TOTP is required") {
                        setShowOtp(true);
                        toast.info("Two-factor authentication required. Please enter your code.");
                    }
                    break;
                case 500:
                    toast.error("Server error. Please try again later.");
                    break;
                default:
                    toast.error(axiosErr?.response?.data?.error || "Failed to reset password.");
            }
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="relative flex flex-col min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
            {/* Main content area - centered vertically and horizontally */}
            <div className="flex-1 flex items-center justify-center safe-px py-8">
                <div className="flex flex-col auth-shell items-center gap-4 px-4 sm:px-0">
                    <Card className="flex flex-col items-center gap-8 p-11 bg-[var(--shadcn-ui-app-popover)] rounded-[var(--primitives-radius-radius-md)] border border-solid border-[var(--shadcn-ui-app-border)] w-full max-w-md mx-auto">
                        <CardContent className="flex items-center gap-8 w-full flex-col p-0">
                            <div className="flex items-center gap-6 flex-col">
                                <div className="flex items-center gap-2 text-center">
                                    <img
                                        className="w-[200px] h-10"
                                        alt="modDNS logo"
                                        src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                        style={{ display: "block" }}
                                    />
                                </div>
                            </div>

                            <div className="flex items-center gap-8 w-full flex-col">
                                <div className="flex items-center gap-4 flex-col">
                                    <div className="w-[190px] h-4 flex items-center justify-center">
                                        <ShadcnLabel className="font-mono font-bold text-[var(--shadcn-ui-app-foreground)] text-xl text-center tracking-[-0.60px] leading-4 whitespace-nowrap">
                                            Set new password
                                        </ShadcnLabel>
                                    </div>

                                    <div className="flex items-center gap-2 flex-col">
                                        <p className="w-72 font-normal text-[var(--shadcn-ui-app-secondary-foreground)] text-sm text-center tracking-[0] leading-[21.1px]">
                                            Your new password must be different to previously used passwords.
                                        </p>
                                    </div>
                                </div>

                                <form className="flex items-start gap-6 w-full flex-col" onSubmit={handleReset}>
                                    <div className="flex items-start gap-8 w-full flex-col">
                                        <div className="flex items-start gap-4 w-full flex-col">
                                            <Input
                                                type="password"
                                                placeholder="New password"
                                                className="flex items-center gap-2 py-2.5 px-3 w-full bg-[var(--shadcn-ui-app-background)] rounded-[var(--primitives-radius-radius-md)] border-0 text-[var(--shadcn-ui-app-muted-foreground)] font-normal text-sm tracking-[0] leading-6"
                                                value={newPassword}
                                                onChange={e => setNewPassword(e.target.value)}
                                                required
                                                disabled={loading}
                                            />

                                            <Input
                                                type="password"
                                                placeholder="Confirm password"
                                                className="flex items-center gap-2 py-2.5 px-3 w-full bg-[var(--shadcn-ui-app-background)] rounded-[var(--primitives-radius-radius-md)] border-0 text-[var(--shadcn-ui-app-muted-foreground)] font-normal text-sm tracking-[0] leading-6"
                                                value={confirmPassword}
                                                onChange={e => setConfirmPassword(e.target.value)}
                                                required
                                                disabled={loading}
                                            />
                                            <p className="text-xs text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                                                {PASSWORD_COMPLEXITY_RULES}
                                            </p>

                                            {showOtp && (
                                                <Input
                                                    type="text"
                                                    placeholder="2FA code"
                                                    className="flex items-center gap-2 py-2.5 px-3 w-full bg-[var(--shadcn-ui-app-background)] rounded-[var(--primitives-radius-radius-md)] border-0 text-[var(--shadcn-ui-app-muted-foreground)] font-normal text-sm tracking-[0] leading-6"
                                                    value={otp}
                                                    onChange={e => setOtp(e.target.value)}
                                                    maxLength={8}
                                                    autoFocus
                                                    disabled={loading}
                                                    required
                                                />
                                            )}
                                        </div>

                                        <div className="flex items-start gap-2 w-full flex-col">
                                            <Button
                                                type="submit"
                                                className="flex min-w-20 items-center justify-center gap-1 py-2 px-3 w-full bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-600)]/90 rounded-[var(--primitives-radius-radius-md)] h-auto font-medium text-sm text-[var(--shadcn-ui-app-background)]"
                                                disabled={loading}
                                            >
                                                {loading ? "Resetting..." : "Reset password"}
                                            </Button>

                                            <Button
                                                type="button"
                                                variant="secondary"
                                                className="flex min-w-16 items-center justify-center py-1.5 px-2 w-full bg-[var(--shadcn-ui-app-secondary)] hover:bg-[var(--tailwind-colors-slate-900)]/90 rounded-[var(--primitives-radius-radius-md)] h-auto gap-2 font-medium text-sm text-[var(--tailwind-colors-rdns-600)]"
                                                onClick={() => navigate("/login")}
                                                disabled={loading}
                                            >
                                                <ArrowLeft className="w-4 h-4" />
                                                Back to log in
                                            </Button>
                                        </div>
                                    </div>
                                </form>
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
