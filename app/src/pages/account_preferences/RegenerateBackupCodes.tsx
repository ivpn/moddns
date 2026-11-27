import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import api from '@/api/api';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import BackupCodesView from '@/components/dialogs/BackupCodesView';
import VerifyIdentityFields from '@/components/dialogs/VerifyIdentityFields';
import { useIdentityVerification } from '@/hooks/useIdentityVerification';
import { totpErrorMessage } from '@/lib/authToasts';
import { beginBackupCodesRegenerateReauth } from '@/lib/webauthn';
import { useAppStore } from '@/store/general';

interface RegenerateBackupCodesDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

/** Replaces every backup code with a new set (api-endpoint-behaviour E13), confirmed with the password or a
 *  passkey and a 2FA code. */
export default function RegenerateBackupCodesDialog({ open, onOpenChange }: RegenerateBackupCodesDialogProps) {
    const { verification, onChange: onVerificationChange, passkeyResetKey, requestFailed, reauthToken } = useIdentityVerification();
    const [submitting, setSubmitting] = useState(false);
    const [codes, setCodes] = useState<string[] | null>(null);
    const account = useAppStore(s => s.account);
    const setAccount = useAppStore(s => s.setAccount);
    // The new codes are shown only once: the dialog stays open until the user confirms they saved them.
    const holdOpen = codes !== null;

    useEffect(() => {
        if (open) setCodes(null);
    }, [open]);

    const refreshAccount = async () => {
        try {
            const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
            setAccount(refreshed.data);
        } catch (error) {
            console.error('Failed to refresh account after regenerating backup codes:', error);
        }
    };

    const regenerate = async () => {
        if (!verification?.otp || submitting) return;
        setSubmitting(true);
        try {
            const res = await api.Client.accountsApi.apiV1AccountsMfaTotpBackupCodesRegeneratePost({
                otp: verification.otp,
                reauth_token: await reauthToken('backup_codes_regenerate'),
            });
            setCodes(res.data.backup_codes ?? []);
            toast.success('New backup codes created. Your old codes no longer work.');
            // Update the banner and the 2FA row behind the dialog now; the old set is already gone.
            void refreshAccount();
        } catch (e: unknown) {
            requestFailed(e);
            const axiosErr = e as { response?: { data?: { error?: string } } };
            toast.error(totpErrorMessage(axiosErr?.response?.data?.error, 'Failed to regenerate backup codes.'));
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={value => { if (!value && holdOpen) return; onOpenChange(value); }}>
            <DialogContent
                data-testid="regenerate-backup-codes-dialog"
                className="w-[95vw] sm:max-w-2xl max-h-[90dvh] overflow-y-auto overscroll-contain p-0 border-[var(--tailwind-colors-slate-600)] bg-[var(--shadcn-ui-app-background)] [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]"
                showCloseButton={!holdOpen}
                onInteractOutside={e => { if (holdOpen) e.preventDefault(); }}
                onEscapeKeyDown={e => { if (holdOpen) e.preventDefault(); }}
            >
                <DialogHeader className="p-4 sm:p-6 pb-0 sm:pb-0 pr-12">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica]">
                        Regenerate backup codes
                    </DialogTitle>
                </DialogHeader>

                <div className="flex flex-col gap-6 p-4 sm:p-6">
                    {codes ? (
                        <BackupCodesView codes={codes} onDone={() => onOpenChange(false)} />
                    ) : (
                        <>
                            <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-50)] leading-5 font-['Roboto_Flex-Regular',Helvetica]">
                                You will get 8 new backup codes. Your current backup codes stop working as soon as the new ones are created. Your authenticator app keeps working as before.
                            </DialogDescription>
                            <VerifyIdentityFields
                                account={account ?? null}
                                open={open}
                                beginPasskeyReauth={beginBackupCodesRegenerateReauth}
                                onChange={onVerificationChange}
                                passkeyResetKey={passkeyResetKey}
                                requireCode
                                onSubmit={regenerate}
                                codeTestId="regenerate-backup-codes-code"
                                autoFocus
                                disabled={submitting}
                            />
                            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-end gap-3">
                                <Button
                                    variant="cancel"
                                    size="lg"
                                    className="flex-1 min-w-32 font-medium"
                                    onClick={() => onOpenChange(false)}
                                    disabled={submitting}
                                >
                                    Cancel
                                </Button>
                                <Button
                                    size="lg"
                                    data-testid="regenerate-backup-codes-submit"
                                    className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)] flex items-center justify-center"
                                    disabled={!verification || submitting}
                                    onClick={regenerate}
                                >
                                    {submitting && <Loader2 className="animate-spin w-4 h-4 mr-2" />}
                                    Regenerate codes
                                </Button>
                            </div>
                        </>
                    )}
                </div>
            </DialogContent>
        </Dialog>
    );
}
