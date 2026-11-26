import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import api from '@/api/api';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import VerifyIdentityFields from '@/components/dialogs/VerifyIdentityFields';
import { useIdentityVerification } from '@/hooks/useIdentityVerification';
import type { ModelAccount } from '@/api/client/api';
import { ModelAccountUpdateOperationEnum, ModelAccountUpdatePathEnum } from '@/api/client/api';
import { useAppStore } from '@/store/general';
import { DialogActions } from '@/components/dialogs/DialogLayout';
import { beginEmailChangeReauth } from '@/lib/webauthn';
import { totpErrorMessage } from '@/lib/authToasts';

interface ChangeEmailDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    account: ModelAccount | null;
    onChanged: () => void; // callback after successful change
}

// Canonical comparison form, mirroring backend email normalization.
const normalizeEmail = (s: string) => s.trim().toLowerCase();

export default function ChangeEmailDialog({ open, onOpenChange, account, onChanged }: ChangeEmailDialogProps) {
    const [step, setStep] = useState<'details' | 'confirm'>('details');
    const [newEmail, setNewEmail] = useState('');
    const [confirmEmail, setConfirmEmail] = useState('');
    interface EmailChangeValuePassword {
        current_password: string;
        new_email: string;
    }
    interface EmailChangeValueReauth {
        reauth_token: string;
        new_email: string;
    }
    const { verification, onChange: onVerificationChange, passkeyResetKey, requestFailed } = useIdentityVerification();
    const [submitting, setSubmitting] = useState(false);
    const setAccount = useAppStore(s => s.setAccount);

    useEffect(() => {
        if (!open) {
            setStep('details');
            setConfirmEmail('');
        }
    }, [open]);

    const emailsMatch = confirmEmail !== '' && normalizeEmail(confirmEmail) === normalizeEmail(newEmail);
    const showMismatch = confirmEmail !== '' && !emailsMatch;

    // Validate step-1 input and advance to the confirm step; the PATCH only
    // happens from the confirm step in handleSubmit.
    const handleContinue = () => {
        if (!account) return;
        if (!newEmail.includes('@')) {
            toast.error('Please enter a valid email.');
            return;
        }
        if (!verification) return;
        setStep('confirm');
    };

    // Handle email change with optional 2FA; reachable only from the confirm step.
    const handleSubmit = async () => {
        if (!account || !verification) return;
        if (!emailsMatch) {
            toast.error('Email addresses do not match.');
            return;
        }

        setSubmitting(true);
        try {
            const value: EmailChangeValuePassword | EmailChangeValueReauth = verification.reauthToken
                ? { reauth_token: verification.reauthToken, new_email: newEmail }
                : { current_password: verification.currentPassword ?? '', new_email: newEmail };
            const req = {
                updates: [
                    {
                        operation: ModelAccountUpdateOperationEnum.Replace,
                        path: ModelAccountUpdatePathEnum.Email,
                        value
                    }
                ]
            };

            await api.Client.accountsApi.apiV1AccountsPatch({ ...req, otp: verification.otp });

            const refreshed = await api.Client.accountsApi.apiV1AccountsCurrentGet();
            setAccount(refreshed.data);
            toast.success('Email updated. Please verify your new address.');
            handleOpenChange(false);
            onChanged();
        } catch (e) {
            requestFailed();
            const err = e as { response?: { data?: { error?: string } } };
            toast.error(totpErrorMessage(err.response?.data?.error, 'Failed to update email'));
        } finally {
            setSubmitting(false);
        }
    };

    const handleOpenChange = (next: boolean) => {
        if (!next) {
            // Reset state when closing
            setStep('details');
            setNewEmail('');
            setConfirmEmail('');
        }
        onOpenChange(next);
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent
                className={`dialog-shell bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] p-0 px-4 sm:px-0
                [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] transition-opacity duration-200 ease-out`}
            >
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Change email
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        {step === 'details' ? 'Enter your new email address below.' : 'Re-enter your new email address to confirm it.'}
                    </DialogDescription>
                </DialogHeader>

                {/* Kept mounted on the confirm step so Back returns to the filled-in verification. */}
                <div className={step === 'details' ? 'flex flex-col gap-4 px-6 pb-4' : 'hidden'}>
                    <div className="space-y-2">
                        <label htmlFor="change-email-new" className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">New email</label>
                        <Input id="change-email-new" value={newEmail} onChange={e => { setNewEmail(e.target.value); setConfirmEmail(''); }} placeholder="new@example.com" autoFocus className="border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-50)] h-10" />
                    </div>
                    <VerifyIdentityFields
                        account={account}
                        open={open}
                        beginPasskeyReauth={beginEmailChangeReauth}
                        onChange={onVerificationChange}
                        passkeyResetKey={passkeyResetKey}
                        onSubmit={handleContinue}
                        codeTestId="change-email-code"
                        disabled={submitting}
                    />
                    <p className="text-xs text-[var(--tailwind-colors-slate-400)]">After changing your email it will be unverified until you complete the new verification OTP process.</p>
                </div>

                {step === 'confirm' && (
                <div className="flex flex-col gap-4 px-6 pb-4">
                    <div className="space-y-2">
                        <label className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">Confirm new email</label>
                        <Input
                            data-testid="input-confirm-email"
                            value={confirmEmail}
                            onChange={e => setConfirmEmail(e.target.value)}
                            onKeyDown={e => {
                                if (e.key === 'Enter' && emailsMatch && !submitting) {
                                    handleSubmit();
                                }
                            }}
                            placeholder="Re-enter your new email"
                            autoFocus
                            className="border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-50)] h-10"
                        />
                        {showMismatch && (
                            <p data-testid="confirm-email-mismatch" className="text-xs text-[var(--tailwind-colors-red-400)]">
                                Email addresses do not match.
                            </p>
                        )}
                    </div>
                    <p
                        data-testid="confirm-email-warning"
                        className="text-xs leading-5 text-[var(--tailwind-colors-amber-400,#fbbf24)] rounded-md border border-[#fbbf24]/30 bg-[#fbbf24]/10 p-3"
                    >
                        Make sure this address is real and you can receive mail at it. If it has a typo or doesn't
                        exist, verification and password reset emails will never reach you, and you could lose
                        access to your account.
                    </p>
                </div>
                )}

                <DialogActions>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => step === 'confirm' ? setStep('details') : handleOpenChange(false)}
                        disabled={submitting}
                    >
                        {step === 'confirm' ? 'Back' : 'Cancel'}
                    </Button>
                    {step === 'details' ? (
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                            onClick={handleContinue}
                            disabled={submitting || !newEmail || !verification}
                        >
                            Continue
                        </Button>
                    ) : (
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                            onClick={handleSubmit}
                            disabled={submitting || !emailsMatch}
                        >
                            {submitting ? 'Updating...' : 'Update email'}
                        </Button>
                    )}
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}