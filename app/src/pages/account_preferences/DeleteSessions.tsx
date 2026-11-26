import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import VerifyIdentityFields from "@/components/dialogs/VerifyIdentityFields";
import { useIdentityVerification } from '@/hooks/useIdentityVerification';
import { useState, type JSX } from "react";
import api from "@/api/api";
import { toast } from "sonner";
import { totpErrorMessage } from "@/lib/authToasts";
import { beginSessionsDeleteReauth, verificationBody } from "@/lib/webauthn";
import { useAppStore } from "@/store/general";

interface DeleteSessionsProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export default function DeleteSessionsDialog({
    open,
    onOpenChange,
}: DeleteSessionsProps): JSX.Element {
    const account = useAppStore(s => s.account);
    const { verification, onChange: onVerificationChange, passkeyResetKey, requestFailed } = useIdentityVerification();
    const [loading, setLoading] = useState(false);

    const handleConfirm = async () => {
        if (!verification || loading) return;
        setLoading(true);
        try {
            await api.Client.sessionsApi.apiV1SessionsDelete(verificationBody(verification));
            toast.success("All other sessions have been logged out.");
            onOpenChange(false);
        } catch (e: unknown) {
            requestFailed();
            const axiosErr = e as { response?: { data?: { error?: string } } };
            toast.error(totpErrorMessage(axiosErr?.response?.data?.error, "Failed to log out sessions."));
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent data-testid="delete-sessions-dialog" className="dialog-shell bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] px-4 sm:px-0">
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Log out other web sessions?
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        This will log out all other web sessions except your current one. You will remain logged in on this device. Confirm it's you to continue.
                    </DialogDescription>
                </DialogHeader>
                <div className="px-6 pb-4">
                    <VerifyIdentityFields
                        account={account ?? null}
                        open={open}
                        beginPasskeyReauth={beginSessionsDeleteReauth}
                        onChange={onVerificationChange}
                        passkeyResetKey={passkeyResetKey}
                        onSubmit={handleConfirm}
                        codeTestId="delete-sessions-code"
                        autoFocus
                        disabled={loading}
                    />
                </div>
                <DialogActions>
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
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                        onClick={handleConfirm}
                        disabled={loading || !verification}
                    >
                        Log out sessions
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
