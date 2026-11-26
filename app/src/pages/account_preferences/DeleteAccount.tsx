import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { DialogActions } from '@/components/dialogs/DialogLayout';
import { type JSX, useState, useEffect, useContext, useCallback } from 'react';
import { toast } from 'sonner';
import API from '@/api/api';
import { AuthContext } from '@/App';
import { useAppStore } from '@/store/general';
import VerifyIdentityFields from '@/components/dialogs/VerifyIdentityFields';
import { useIdentityVerification } from '@/hooks/useIdentityVerification';
import { beginAccountDeletionReauth, verificationHeaders } from '@/lib/webauthn';
import { totpErrorMessage } from '@/lib/authToasts';
import type { RequestsAccountDeletionRequest } from '@/api/client/api';

interface DeleteAccountProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    loading?: boolean;
}

export default function DeleteAccountDialog({
    open,
    onOpenChange,
    loading = false,
}: DeleteAccountProps): JSX.Element {
    const account = useAppStore(s => s.account);
    const { verification, onChange: onVerificationChange, passkeyResetKey, requestFailed } = useIdentityVerification();
    const [deletionCode, setDeletionCode] = useState<string>('');
    const [userCode, setUserCode] = useState<string>('');
    const [isGenerating, setIsGenerating] = useState(false);
    const [isDeleting, setIsDeleting] = useState(false);
    const [isCopied, setIsCopied] = useState(false);
    const [hasAttemptedGeneration, setHasAttemptedGeneration] = useState(false);
    const auth = useContext(AuthContext);

    const generateDeletionCode = useCallback(async () => {
        setIsGenerating(true);
        setHasAttemptedGeneration(true);
        try {
            const response = await API.Client.accountsApi.apiV1AccountsCurrentDeletionCodePost({});
            if (response.data && response.data.code) {
                setDeletionCode(response.data.code);
                setIsCopied(false);
            } else {
                toast.error('Failed to generate deletion code.');
            }
        } catch (err) {
            console.error('Error generating deletion code:', err);
            const error = err as { response?: { status?: number; data?: { error?: string } } };
            if (error.response?.status === 429) {
                toast.error('Too many requests, please try again later.');
            } else if (error.response?.data?.error) {
                toast.error(error.response.data.error);
            } else {
                toast.error('Failed to generate deletion code.');
            }
        } finally {
            setIsGenerating(false);
        }
    }, []);

    useEffect(() => {
        if (!open) {
            setDeletionCode('');
            setUserCode('');
            setIsCopied(false);
            setHasAttemptedGeneration(false);
            setIsGenerating(false);
        }
    }, [open]);

    useEffect(() => {
        if (!open || deletionCode || isGenerating || hasAttemptedGeneration) {
            return;
        }
        void generateDeletionCode();
    }, [open, deletionCode, isGenerating, hasAttemptedGeneration, generateDeletionCode]);

    const handleConfirm = async () => {
        if (!userCode.trim()) {
            toast.error('Please enter the deletion code.');
            return;
        }

        if (!verification) return;

        const payload: RequestsAccountDeletionRequest = {
            deletion_code: userCode.trim(),
            current_password: verification.currentPassword,
            reauth_token: verification.reauthToken,
        };

        setIsDeleting(true);
        try {
            await API.Client.accountsApi.apiV1AccountsCurrentDelete(payload, { headers: verificationHeaders(verification) });
            onOpenChange(false);
            auth?.logout('Account deleted.', 'info');
        } catch (err) {
            requestFailed();
            const error = err as { response?: { status?: number; data?: { error?: string } } };
            if (error.response?.status === 429) {
                toast.error('Too many requests, please try again later.');
            } else if (error.response?.status === 400 && error.response?.data?.error === 'invalid deletion code') {
                toast.error('Failed to delete account - invalid or expired deletion code.');
            } else {
                toast.error(totpErrorMessage(error.response?.data?.error, 'Unknown error, please try again later.'));
            }
        } finally {
            setIsDeleting(false);
        }
    };

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(deletionCode);
            setIsCopied(true);
            toast.success("Deletion code copied to clipboard.");

            // Reset the button text after 2 seconds
            setTimeout(() => {
                setIsCopied(false);
            }, 2000);
        } catch {
            toast.error('Failed to copy to clipboard.');
        }
    };

    // Reset state when dialog is closed
    const handleOpenChange = (open: boolean) => {
        if (!open) {
            setDeletionCode('');
            setUserCode('');
            setIsCopied(false);
            setHasAttemptedGeneration(false);
            setIsGenerating(false);
        }
        onOpenChange(open);
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="w-full max-w-[calc(100vw-2rem)] sm:max-w-[500px] border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]">
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Confirm account deletion
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        This will permanently delete your account and all associated data. This action cannot be undone.
                    </DialogDescription>
                </DialogHeader>

                <div className="px-6 pb-4 space-y-4">
                    <VerifyIdentityFields
                        account={account ?? null}
                        open={open}
                        beginPasskeyReauth={beginAccountDeletionReauth}
                        onChange={onVerificationChange}
                        passkeyResetKey={passkeyResetKey}
                        codeTestId="delete-account-code"
                        autoFocus
                        disabled={isDeleting}
                    />

                    {isGenerating ? (
                        <div className="space-y-2">
                            <p className="text-sm text-[var(--tailwind-colors-slate-400)]">Generating deletion code...</p>
                            <div className="flex justify-center py-4">
                                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-[var(--tailwind-colors-rdns-600)]"></div>
                            </div>
                        </div>
                    ) : deletionCode ? (
                        <div className="space-y-4">
                            <div className="space-y-2">
                                <Label htmlFor="deletion-code" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    Your deletion code:
                                </Label>
                                <div className="relative">
                                    <Input
                                        id="deletion-code"
                                        type="text"
                                        value={deletionCode}
                                        readOnly
                                        className="bg-[var(--tailwind-colors-slate-700)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)] font-mono text-center"
                                    />
                                    <Button
                                        type="button"
                                        className="absolute right-2 top-1/2 -translate-y-1/2 h-6 px-2 text-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-slate-600)] hover:bg-[var(--tailwind-colors-slate-500)] text-xs cursor-pointer"
                                        onClick={handleCopy}
                                        disabled={isCopied}
                                    >
                                        {isCopied ? "Copied!" : "Copy"}
                                    </Button>
                                </div>
                                <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                    Copy this code and enter it below to confirm deletion after verification.
                                </p>
                            </div>

                            <div className="space-y-2">
                                <Label htmlFor="user-code" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    Enter deletion code to confirm:
                                </Label>
                                <Input
                                    id="user-code"
                                    type="text"
                                    value={userCode}
                                    onChange={(e) => setUserCode(e.target.value)}
                                    placeholder="Enter the deletion code"
                                    className="bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)] placeholder-[var(--tailwind-colors-slate-400)]"
                                />
                            </div>
                        </div>
                    ) : (
                        <div className="space-y-2">
                            <p className="text-sm text-[var(--tailwind-colors-slate-400)]">
                                We could not generate a deletion code automatically. Please retry.
                            </p>
                            <Button
                                onClick={() => void generateDeletionCode()}
                                disabled={isGenerating}
                                className="w-full h-auto py-2 px-4 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-700)] cursor-pointer disabled:opacity-60"
                            >
                                <span className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    Retry generating code
                                </span>
                            </Button>
                        </div>
                    )}
                </div>

                <DialogActions>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => handleOpenChange(false)}
                        disabled={loading || isGenerating || isDeleting}
                    >
                        Cancel
                    </Button>
                    <Button
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                        onClick={handleConfirm}
                        disabled={
                            loading ||
                            isGenerating ||
                            isDeleting ||
                            !deletionCode ||
                            !userCode.trim() ||
                            !verification
                        }
                    >
                        {isDeleting ? 'Deleting...' : 'Delete account'}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
