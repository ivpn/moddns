import { useState, useEffect, useContext } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import api from "@/api/api";
import LoginCard from "@/pages/auth/LoginCard";
import AuthFooter from "@/components/auth/AuthFooter";
import { Alert } from "@/components/ui/alert";
import { Info } from "lucide-react";
import { authToasts } from "@/lib/authToasts";
import { AuthContext } from "@/App";
import { authenticateWithPasskey, isWebAuthnSupported } from "@/lib/webauthn";
import SessionLimitDialog from "@/components/dialogs/SessionLimitDialog";

const infoAlertData = {
    title: "Here to try modDNS? You need an active IVPN account.",
    linkUrl: "https://ivpn.net/account/",
    linkText: "ivpn.net",
};

export default function Login() {
    const location = useLocation();
    const navigate = useNavigate();
    const auth = useContext(AuthContext); // Move this to top level
    const [, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [webAuthnSupported] = useState(() => isWebAuthnSupported());

    // 2FA state
    const [showOtp, setShowOtp] = useState(false);
    const [pendingEmail, setPendingEmail] = useState("");
    const [pendingPassword, setPendingPassword] = useState("");

    // Session limit dialog state
    const [showSessionLimitDialog, setShowSessionLimitDialog] = useState(false);
    const [sessionLimitLoading, setSessionLimitLoading] = useState(false);

    useEffect(() => {
        if (location.state?.passwordResetSuccess) {
            authToasts.passwordResetSuccess();
        }
    }, [location.state]);

    // Show any deferred email verification toasts (success or error variants) based on sessionStorage flags
    useEffect(() => {
        const keyMap: Record<string, () => void> = {
            emailVerifiedShowToast: () => authToasts.emailVerifiedSuccess(),
            emailVerifyInvalid: () => authToasts.emailVerifyInvalid(),
            emailVerifyExpired: () => authToasts.emailVerifyExpired(),
            emailVerifyAlready: () => authToasts.emailVerifiedAlready(),
            emailVerifyGenericError: () => authToasts.emailVerifyGenericError(),
            emailVerifyRateLimit: () => authToasts.emailVerifyRateLimit(),
        };
        let triggered = false;
        Object.entries(keyMap).forEach(([k, fn]) => {
            if (sessionStorage.getItem(k) === '1') {
                fn();
                sessionStorage.removeItem(k);
                triggered = true;
            }
        });
        if (triggered) {
            // Optionally could focus the form or log analytics; no-op for now
        }
    }, []);

    useEffect(() => {
        const searchParams = new URLSearchParams(location.search);
        if (searchParams.get('account_deleted') === 'true') {
            authToasts.accountDeletedSuccess();
            // Clean up the URL parameter
            searchParams.delete('account_deleted');
            const newUrl = `${location.pathname}${searchParams.toString() ? '?' + searchParams.toString() : ''}`;
            navigate(newUrl, { replace: true });
        }
    }, [location.search, location.pathname, navigate]);

    const handleRemoveAllSessions = async () => {
        // For passkey login, we only need email. For regular login, we need both email and password
        if (!pendingEmail || (!pendingPassword && pendingPassword !== "")) {
            return;
        }

        try {
            setSessionLimitLoading(true);

            // Check if this is a passkey login (empty password) or regular login
            if (pendingPassword === "") {
                // Passkey login - use session removal
                await authenticateWithPasskey(pendingEmail, undefined, true);

                // If we reach here, authentication was successful and verified
                auth?.login();
            } else {
                // Regular login - use existing logic
                const data = {
                    email: pendingEmail,
                    password: pendingPassword
                };

                // Try login again with session removal header set to "true"
                const response = await api.Client.authApi.apiV1LoginPost(
                    data,
                    undefined, // otp
                    undefined, // mfaMethods  
                    "true"     // xRemoveSessions
                );

                if (response.status !== 200) {
                    throw new Error("Login failed");
                }

                auth?.login();
            }
            // Always redirect to home page after successful login
            navigate("/home", { replace: true });
        } catch {
            setError("Failed to login. Please try again.");
        } finally {
            setShowSessionLimitDialog(false);
            setPendingEmail("");
            setPendingPassword("");
            setSessionLimitLoading(false);
        }
    };

    const handleCancelSessionLimit = () => {
        setShowSessionLimitDialog(false);
        setPendingEmail("");
        setPendingPassword("");
    };

    // Handles login with or without OTP
    const handleLogin = async (email: string, password: string, otp?: string) => {
        setLoading(true);
        setError(null);

        const data = { email, password };
        const mfaMethods = ["totp"];
        const xRemoveSessions = "false";

        try {
            const response = await api.Client.authApi.apiV1LoginPost(
                data,
                otp,
                mfaMethods,
                xRemoveSessions
            );

            if (response.status === 200) {
                auth?.login();
                // Always redirect to home page after successful login
                navigate("/home", { replace: true });
            } else {
                setError("Invalid credentials or login failed.");
                authToasts.invalidCredentials();
            }
        } catch (err: unknown) {
            const axiosErr = err as { response?: { status?: number; data?: { error?: string } } };
            if (
                axiosErr?.response?.status === 401
            ) {
                switch (axiosErr?.response?.data?.error) {
                    case "TOTP is required":
                    case "TOTP_REQUIRED":
                        authToasts.totpRequired();
                        setShowOtp(true);
                        setPendingEmail(email);
                        setPendingPassword(password);
                        setError("Two-factor authentication required. Please enter your code.");
                        break;
                    default:
                        setError("Unauthorized. Please check your credentials.");
                        authToasts.unauthorized();
                }
            } else if (axiosErr?.response?.status === 429) {
                // Check if it's session limit error
                if (axiosErr?.response?.data?.error === "maximum number of active sessions reached") {
                    setShowSessionLimitDialog(true);
                    setPendingEmail(email);
                    setPendingPassword(password);
                } else {
                    setError("Too many login attempts. Please try again later.");
                    authToasts.tooManyAttempts();
                }
            } else if (axiosErr?.response?.status === 400) {
                const apiErr = axiosErr?.response?.data?.error || '';
                if (apiErr.toLowerCase().includes('invalid')) {
                    setError("Invalid credentials or login failed.");
                    authToasts.invalidCredentials();
                } else {
                    setError(apiErr || "An unexpected error occurred.");
                    authToasts.unexpectedError(apiErr);
                }
            } else {
                const apiErr = axiosErr?.response?.data?.error;
                setError(apiErr || "An unexpected error occurred.");
                authToasts.unexpectedError(apiErr);
            }
        } finally {
            setLoading(false);
        }
    };

    // Handler for OTP submit
    const handleOtpLogin = async (otp: string) => {
        await handleLogin(pendingEmail, pendingPassword, otp);
    };

    // Handler for passkey authentication
    const handlePasskeyLogin = async (email: string) => {
        setLoading(true);
        setError(null);

        try {
            let sessionLimitDialogShown = false;

            await authenticateWithPasskey(email, () => {
                // Handle session limit reached
                sessionLimitDialogShown = true;
                setShowSessionLimitDialog(true);
                setPendingEmail(email);
                setPendingPassword(""); // No password for passkey login
            });

            // If we reach here, authentication was successful and verified
            // Only suppress toast if session limit dialog was shown, otherwise show success toast
            auth?.login(!sessionLimitDialogShown);

            // Always redirect to home page after successful login
            navigate("/home", { replace: true });
        } catch (err: unknown) {
            const errorMessage = (err instanceof Error ? err.message : null) || "Passkey authentication failed";
            setError(errorMessage);
            authToasts.passkeyError(errorMessage);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div data-testid="login-page" className="relative flex flex-col min-h-screen w-full overflow-x-hidden bg-[var(--public-page-background)]">
            {/* Main content area - centered vertically and horizontally */}
            <div className="flex-1 flex items-center justify-center safe-px py-8">
                <div className="flex flex-col auth-shell items-end gap-4 px-4 sm:px-0">
                    <LoginCard
                        onLogin={showOtp
                            ? async (_email, _password, otp) => handleOtpLogin(otp || "")
                            : async (email, password) => handleLogin(email, password)
                        }
                        onPasskeyLogin={handlePasskeyLogin}
                        loading={loading}
                        showOtp={showOtp}
                        initialPasskeyMode={webAuthnSupported}
                    />

                    {/* Info alert */}
                    <Alert className="bg-[var(--alert-card-bg)] border border-[var(--alert-card-bg)] rounded-lg">
                        <Info className="h-[18px] w-[18px] text-[var(--alert-card-icon)]" />
                        <div className="flex flex-col gap-2">
                            <h4 className="text-sm leading-5 font-semibold text-[var(--alert-card-fg)]">
                                Here to try modDNS? You need an active IVPN account.
                            </h4>
                            <div className="text-sm leading-5 text-[var(--alert-card-fg)] opacity-80">
                                Sign up or log in on{" "}
                                <a
                                    href={infoAlertData.linkUrl}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="inline p-0 m-0 h-auto align-baseline !underline !text-[var(--tailwind-colors-rdns-600)] hover:!text-[var(--tailwind-colors-rdns-700)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]"
                                    aria-label={`Open ${infoAlertData.linkText} in a new tab`}
                                >
                                    {infoAlertData.linkText}
                                </a>{" "}
                                and look for "DNS Beta" in your account settings.
                            </div>
                        </div>
                    </Alert>
                </div>
            </div>

            {/* AuthFooter pinned to bottom with proper spacing */}
            <div className="w-full px-4 pb-8 pt-16">
                <AuthFooter />
            </div>

            {/* Session Limit Dialog */}
            <SessionLimitDialog
                open={showSessionLimitDialog}
                onOpenChange={(open) => {
                    if (!open) {
                        handleCancelSessionLimit();
                    }
                }}
                onConfirm={handleRemoveAllSessions}
                loading={sessionLimitLoading}
            />
        </div >
    );
}
