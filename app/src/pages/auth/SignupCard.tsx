import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg'
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg'
import { useNavigate } from "react-router-dom";
import { useTheme } from "@/components/theme-provider";

import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Mail, Lock, Eye, EyeOff, Key } from "lucide-react";
import { type JSX, useState, useEffect } from "react";
import { isWebAuthnSupported } from "@/lib/webauthn";

// Data for the signup form
const signupData = {
    title: "Create your account",
    emailPlaceholder: "Email address",
    passwordPlaceholder: "Password",
    signupText: "Sign Up",
    loginText: "Already have an account? Log in",
};

interface SignupCardProps {
    onSignup?: (email: string, password: string) => void | Promise<void>;
    onPasskeySignup?: (email: string) => void | Promise<void>;
    loading?: boolean;
    error?: string | null;
}

const SignupCard = ({ onSignup, onPasskeySignup, loading = false, error }: SignupCardProps): JSX.Element => {
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [passkeyEmail, setPasskeyEmail] = useState("");
    const [showPassword, setShowPassword] = useState(false);
    const [webAuthnSupported, setWebAuthnSupported] = useState(false);
    const navigate = useNavigate();
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);

    useEffect(() => {
        setWebAuthnSupported(isWebAuthnSupported());
    }, []);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (onSignup) {
            await onSignup(email, password);
        }
    };

    const handlePasskeySignup = async () => {
        if (onPasskeySignup && passkeyEmail) {
            await onPasskeySignup(passkeyEmail);
        }
    };

    const isFormValid = email && password;
    const isPasskeyEmailValid = passkeyEmail && passkeyEmail.includes('@');

    return (
        <Card className="w-full bg-[var(--shadcn-ui-app-popover)] border-[var(--shadcn-ui-app-border)] ">
            <CardContent className="flex flex-col items-center gap-8 p-11">
                {/* Logo */}
                <div className="inline-flex flex-col items-center gap-4 relative">
                    <img
                        className="mb-8 w-[200px] h-10 mx-auto"
                        alt="modDNS logo"
                        src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                    />
                    <h2 className="font-bold text-[var(--shadcn-ui-app-foreground)] text-xl text-center tracking-[-0.60px] leading-[18px] font-mono">
                        {signupData.title}
                    </h2>
                </div>

                {/* Passkey Registration Option */}
                {webAuthnSupported && onPasskeySignup && (
                    <div className="w-full space-y-4">
                        {/* Passkey Email Input */}
                        <div className="space-y-2">
                            <div className="relative">
                                <Mail className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-[var(--shadcn-ui-app-muted-foreground)]" />
                                <Input
                                    id="passkey-email"
                                    type="email"
                                    placeholder="Email address for passkey signup"
                                    value={passkeyEmail}
                                    onChange={(e) => setPasskeyEmail(e.target.value)}
                                    className="pl-10 bg-[var(--shadcn-ui-app-input)] border-[var(--shadcn-ui-app-border)] text-[var(--shadcn-ui-app-foreground)] placeholder:text-[var(--shadcn-ui-app-muted-foreground)]"
                                    disabled={loading}
                                />
                            </div>
                        </div>

                        <Button
                            type="button"
                            onClick={handlePasskeySignup}
                            disabled={loading || !isPasskeyEmailValid}
                            className="w-full bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-800)] cursor-pointer text-[var(--tailwind-colors-slate-900)] font-medium py-2 px-4 rounded-md transition-colors min-h-11 lg:min-h-0"
                        >
                            <Key className="h-4 w-4 mr-2" />
                            {loading ? "Setting up passkey..." : "Sign up with passkey"}
                        </Button>
                        <p className="text-xs text-[var(--shadcn-ui-app-muted-foreground)] text-center leading-relaxed">
                            Create an account using your device's built-in authentication.<br />
                            More secure and convenient than passwords.
                        </p>
                    </div>
                )}

                {/* Separator */}
                {webAuthnSupported && onPasskeySignup && (
                    <div className="w-full flex items-center gap-4">
                        <Separator className="flex-1" />
                        <span className="text-xs text-[var(--shadcn-ui-app-muted-foreground)]">OR</span>
                        <Separator className="flex-1" />
                    </div>
                )}

                {/* Form */}
                <form onSubmit={handleSubmit} className="w-full space-y-6">
                    {/* Email Input */}
                    <div className="space-y-2">
                        <div className="relative">
                            <Mail className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-[var(--shadcn-ui-app-muted-foreground)]" />
                            <Input
                                id="email"
                                type="email"
                                placeholder={signupData.emailPlaceholder}
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                className="pl-10 bg-[var(--shadcn-ui-app-input)] border-[var(--shadcn-ui-app-border)] text-[var(--shadcn-ui-app-foreground)] placeholder:text-[var(--shadcn-ui-app-muted-foreground)]"
                                required
                                disabled={loading}
                            />
                        </div>
                    </div>

                    {/* Password Input */}
                    <div className="space-y-2">
                        <div className="relative">
                            <Lock className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-[var(--shadcn-ui-app-muted-foreground)]" />
                            <Input
                                id="password"
                                type={showPassword ? "text" : "password"}
                                placeholder={signupData.passwordPlaceholder}
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                className="pl-10 pr-10 bg-[var(--shadcn-ui-app-input)] text-[var(--shadcn-ui-app-foreground)] placeholder:text-[var(--shadcn-ui-app-muted-foreground)] border-[var(--shadcn-ui-app-border)]"
                                required
                                disabled={loading}
                            />
                            <button
                                type="button"
                                onClick={() => setShowPassword(!showPassword)}
                                className="absolute right-2 top-1/2 -translate-y-1/2 inline-flex items-center justify-center text-[var(--shadcn-ui-app-muted-foreground)] hover:text-[var(--shadcn-ui-app-foreground)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] rounded-md min-h-11 lg:min-h-0 min-w-11"
                                disabled={loading}
                            >
                                {showPassword ? (
                                    <EyeOff className="h-4 w-4" />
                                ) : (
                                    <Eye className="h-4 w-4" />
                                )}
                            </button>
                        </div>
                        <p className="text-xs text-[var(--shadcn-ui-app-muted-foreground)] leading-relaxed">
                            Password must be 12-64 characters, contain at least one uppercase letter, one lowercase letter, one number, and one special character.
                        </p>
                    </div>

                    {/* Submit Button */}
                    <Button
                        type="submit"
                        variant="outline"
                        className="w-full border-[var(--shadcn-ui-app-border)] text-[var(--shadcn-ui-app-foreground)] hover:bg-[var(--variable-collection-surface)] transition-colors min-h-11 lg:min-h-0"
                        disabled={loading || !isFormValid}
                    >
                        {loading ? "Creating account..." : signupData.signupText}
                    </Button>
                </form>

                {/* Error Display */}
                {error && (
                    <div className="w-full p-3 bg-[var(--tailwind-colors-red-950)] border border-[var(--tailwind-colors-red-600)] rounded-md">
                        <p className="text-sm text-[var(--tailwind-colors-red-400)]">{error}</p>
                    </div>
                )}

                {/* Login Link */}
                <div className="text-center">
                    <button
                        onClick={() => navigate("/login")}
                        className="text-sm text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] cursor-pointer transition-colors inline-flex items-center min-h-11 lg:min-h-0"
                        disabled={loading}
                    >
                        {signupData.loginText}
                    </button>
                </div>
            </CardContent>
        </Card>
    );
};

export default SignupCard;
