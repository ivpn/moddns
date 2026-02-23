import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useAuth } from "@/App";

const isUUIDv4 = (id: string): boolean => {
    const uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;
    return uuidRegex.test(id);
};
import api from "@/api/api";
import SignupCard from "@/pages/auth/SignupCard";
import AuthFooter from "@/components/auth/AuthFooter";
import { registerPasskey } from "@/lib/webauthn";
import { authToasts } from "@/lib/authToasts";

import NotFound from "@/pages/NotFound";

export default function Signup() {
    const navigate = useNavigate();
    const { subid } = useParams();
    const { isAuthenticated } = useAuth();
    const validSubId = (subid && isUUIDv4(subid));
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (isAuthenticated) {
            navigate("/home", { replace: true });
        }
    }, [isAuthenticated, navigate]);

    if (!validSubId) {
        // Missing or invalid subscription id -> 404
        return <NotFound />;
    }

    const handleSignup = async (email: string, password: string) => {
        setLoading(true);
        setError(null);

        try {
            const response = await api.Client.accountsApi.apiV1AccountsPost({
                email,
                password,
                subid: subid!,
            });

            if (response.status === 201) {
                navigate("/login", { replace: true });
                // Use unified toast helper
                authToasts.accountCreatedSuccess();
            }
        } catch (err) {
            const e = err as { response?: { data?: { error?: string; message?: string; details?: unknown } }; message?: string };
            const data = e.response?.data;
            let errorMessage =
                (typeof data?.error === 'string' ? data?.error : undefined) ||
                (typeof data?.message === 'string' ? data?.message : undefined) ||
                (Array.isArray(data?.details) ? (data?.details as string[]).join(', ') : undefined) ||
                e?.message ||
                "Failed to create account";

            if (Array.isArray(data?.details) && (data?.details as unknown[]).some(d => typeof d === 'string' && d.toLowerCase().includes('password'))) {
                errorMessage = "Password must be 12-64 characters, contain at least one uppercase letter, one lowercase letter, one number, and one special character.";
            }

            setError(errorMessage);
            authToasts.unexpectedError(errorMessage);
        } finally {
            setLoading(false);
        }
    };

    const handlePasskeySignup = async (email: string) => {
        setLoading(true);
        setError(null);

        try {
            // Register the passkey first - supply subscription id to backend flow if supported
            // Passkey registration currently does not accept subscription id; will be linked after OpenAPI update
            await registerPasskey(email, subid!);
            navigate("/login", { replace: true });
            authToasts.accountCreatedSuccess();
        } catch (err) {
            const e = err as { response?: { data?: { error?: string } }; message?: string };
            let errorMessage = "Failed to create account with passkey";
            if (e.message && e.message.includes("passkey")) {
                errorMessage = e.message;
            } else if (typeof e?.response?.data?.error === 'string') {
                errorMessage = e.response!.data!.error!;
            } else if (e.message) {
                errorMessage = e.message;
            }

            setError(errorMessage);
            authToasts.unexpectedError(errorMessage);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="relative flex flex-col min-h-screen w-full overflow-x-hidden bg-[var(--shadcn-ui-app-background)]">
            {/* Main content area - centered vertically and horizontally */}
            <div className="flex-1 flex items-center justify-center safe-px py-8">
                <div className="flex flex-col auth-shell items-end gap-4 px-4 sm:px-0">
                    <SignupCard
                        onSignup={handleSignup}
                        onPasskeySignup={handlePasskeySignup}
                        loading={loading}
                        error={error}
                    />
                </div>
            </div>

            {/* AuthFooter pinned to bottom with proper spacing */}
            <div className="w-full px-4 pb-8 pt-16">
                <AuthFooter />
            </div>
        </div>
    );
}
