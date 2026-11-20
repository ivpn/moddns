import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useState, useEffect } from "react";
import { toast } from "sonner";
import api from "@/api/api";
import { Loader2 } from "lucide-react";
import { useAppStore } from "@/store/general";

// An authenticator code has 6 digits; a backup code has at least 8 characters
// and may be entered grouped with hyphens or spaces, which the API ignores.
const MIN_BACKUP_CODE_LENGTH = 8;
const MAX_CODE_LENGTH = 64;

interface Disable2FADialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onDisabled?: () => void;
}

export default function Disable2FADialog({ open, onOpenChange, onDisabled }: Disable2FADialogProps) {
    const [otp, setOtp] = useState("");
    const [loading, setLoading] = useState(false);
    const setAccount = useAppStore(s => s.setAccount);
    const code = otp.replace(/[\s-]/g, "");
    const canSubmit = /^\d{6}$/.test(code) || code.length >= MIN_BACKUP_CODE_LENGTH;

    // Reset OTP every time dialog is opened
    useEffect(() => {
        if (open) setOtp("");
    }, [open]);

    const handleDisable2FA = async () => {
        setLoading(true);
        try {
            const body = { otp } as unknown as { otp: string };
            await api.Client.accountsApi.apiV1AccountsMfaTotpDisablePost(body);
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
            toast.error(axiosErr?.response?.data?.error || "Failed to disable 2FA.");
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
                        Enter the 6-digit code from your authenticator app, or one of your backup codes, to disable 2FA for your account.
                    </p>
                    <div className="flex flex-col items-start gap-2 w-full">
                        <label
                            htmlFor="disable-totp-code"
                            className="text-sm font-medium text-[var(--tailwind-colors-slate-50)] leading-5 font-['Roboto_Flex-Medium',Helvetica]"
                        >
                            Authenticator or backup code
                        </label>
                        <Input
                            id="disable-totp-code"
                            className="h-10 bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-700)] rounded-md"
                            value={otp}
                            onChange={e => setOtp(e.target.value)}
                            disabled={loading}
                            autoFocus
                            autoComplete="one-time-code"
                            autoCapitalize="none"
                            autoCorrect="off"
                            spellCheck={false}
                            maxLength={MAX_CODE_LENGTH}
                            onKeyDown={e => {
                                if (
                                    e.key === "Enter" &&
                                    !loading &&
                                    canSubmit
                                ) {
                                    handleDisable2FA();
                                }
                            }}
                        />
                        <p className="text-sm text-[var(--tailwind-colors-slate-400)] leading-5 font-text-sm-leading-5-normal">
                            6-digit code or a backup code
                        </p>
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
                        disabled={loading || !canSubmit}
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