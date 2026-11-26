import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import VerifyIdentityFields, { type IdentityVerification } from "@/components/dialogs/VerifyIdentityFields";
import { useCallback, useState } from "react";
import { toast } from "sonner";
import { totpErrorMessage } from "@/lib/authToasts";
import { beginTotpDisableReauth } from "@/lib/webauthn";
import api from "@/api/api";
import { Loader2 } from "lucide-react";
import { useAppStore } from "@/store/general";

interface Disable2FADialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onDisabled?: () => void;
}

export default function Disable2FADialog({ open, onOpenChange, onDisabled }: Disable2FADialogProps) {
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const onVerificationChange = useCallback((v: IdentityVerification | null) => setVerification(v), []);
    const [loading, setLoading] = useState(false);
    const account = useAppStore(s => s.account);
    const setAccount = useAppStore(s => s.setAccount);

    const handleDisable2FA = async () => {
        if (!verification?.otp || loading) return;
        setLoading(true);
        try {
            await api.Client.accountsApi.apiV1AccountsMfaTotpDisablePost({
                otp: verification.otp,
                current_password: verification.currentPassword,
                reauth_token: verification.reauthToken,
            });
            toast.success("2FA disabled successfully.");

            // Update the global account state with latest 2FA status
            try {
                const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
                setAccount(refreshed.data);
            } catch (error) {
                // Log error but don't block the flow
                console.error('Failed to refresh account after disabling 2FA:', error);
            }

            if (onDisabled) onDisabled();
            onOpenChange(false);
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { error?: string } } };
            toast.error(totpErrorMessage(axiosErr?.response?.data?.error, "Failed to disable 2FA."));
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="w-lg p-0 border-[var(--tailwind-colors-slate-600)] bg-[var(--shadcn-ui-app-background)] [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]">
                <DialogHeader className="p-6 pb-0 pr-12">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica]">
                        Disable 2-Factor Authentication
                    </DialogTitle>
                </DialogHeader>
                <div className="flex flex-col items-start gap-6 p-6">
                    <p className="text-sm font-normal text-[var(--tailwind-colors-slate-50)] leading-5 font-['Roboto_Flex-Regular',Helvetica]">
                        Confirm it's you, then enter the 6-digit code from your authenticator app to disable 2FA for your account. If you lost your device, use one of your backup codes instead.
                    </p>
                    <div className="w-full">
                        <VerifyIdentityFields
                            account={account ?? null}
                            open={open}
                            beginPasskeyReauth={beginTotpDisableReauth}
                            onChange={onVerificationChange}
                            requireCode
                            onSubmit={handleDisable2FA}
                            codeTestId="disable-2fa-code"
                            autoFocus
                            disabled={loading}
                        />
                    </div>
                </div>
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-end gap-3 p-6 pt-0">
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => onOpenChange(false)}
                        disabled={loading}
                    >
                        Cancel
                    </Button>
                    <Button
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] flex items-center justify-center text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-700)]"
                        disabled={loading || !verification}
                        onClick={handleDisable2FA}
                    >
                        {loading ? (
                            <Loader2 className="animate-spin w-4 h-4 mr-2" />
                        ) : null}
                        Disable 2FA
                    </Button>
                </div>
            </DialogContent>
        </Dialog>
    );
}