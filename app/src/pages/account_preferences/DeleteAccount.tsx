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
import { cn } from '@/lib/utils';
import { useAppStore } from '@/store/general';
import { useAccountVerificationMethod } from '@/hooks/useAccountVerificationMethod';
import { beginAccountDeletionReauth } from '@/lib/webauthn';
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
    const [currentPassword, setCurrentPassword] = useState('');
    const [otp, setOtp] = useState('');
    const [reauthToken, setReauthToken] = useState<string | null>(null);
    const [reauthStatus, setReauthStatus] = useState<'idle' | 'in-progress' | 'verified' | 'error'>('idle');
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

    const handleMethodChange = useCallback(() => {
        setCurrentPassword('');
        setOtp('');
        setReauthToken(null);
        setReauthStatus('idle');
        setDeletionCode('');
        setUserCode('');
        setIsCopied(false);
        setIsGenerating(false);
        setHasAttemptedGeneration(false);
    }, []);

    const {
        method,
        hasPasskeys,
        passwordAvailable,
        showOtp,
        switchMethod,
        resetMethod,
    } = useAccountVerificationMethod({
        account: account ?? null,
        open,
        onMethodChange: handleMethodChange,
    });

    useEffect(() => {
        if (!open) {
            setDeletionCode('');
            setUserCode('');
            setIsCopied(false);
            setCurrentPassword('');
            setOtp('');
            setReauthToken(null);
            setReauthStatus('idle');
            setHasAttemptedGeneration(false);
            setIsGenerating(false);
            resetMethod();
        }
    }, [open, resetMethod]);

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

        if (method === 'password') {
            if (!passwordAvailable) {
                toast.error('Password verification is not available.');
                return;
            }
            if (!currentPassword.trim()) {
                toast.error('Current password is required.');
                return;
            }
            if (showOtp && !otp.trim()) {
                toast.error('2FA code is required.');
                return;
            }
        }

        if (method === 'passkey') {
            if (!hasPasskeys) {
                toast.error('Passkey verification is not available.');
                return;
            }
            if (!reauthToken) {
                toast.error('Passkey verification is required.');
                return;
            }
        }

        const payload: RequestsAccountDeletionRequest = {
            deletion_code: userCode.trim(),
        };

        if (method === 'password') {
            payload.current_password = currentPassword.trim();
        }
        if (method === 'passkey' && reauthToken) {
            payload.reauth_token = reauthToken;
        }

        setIsDeleting(true);
        try {
            const headers: Record<string, string> = {};
            if (method === 'password' && showOtp && otp.trim()) {
                headers['x-mfa-code'] = otp.trim();
                headers['x-mfa-methods'] = 'totp';
            }

            await API.Client.accountsApi.apiV1AccountsCurrentDelete(payload, { headers });
            onOpenChange(false);
            auth?.logout('Account deleted.', 'info');
        } catch (err) {
            const error = err as { response?: { status?: number; data?: { error?: string } } };
            if (error.response?.status === 429) {
                toast.error('Too many requests, please try again later.');
            } else if (error.response?.status === 400 && error.response?.data?.error === 'invalid deletion code') {
                toast.error('Failed to delete account - invalid or expired deletion code.');
            } else if (error.response?.data?.error) {
                toast.error(error.response.data.error);
            } else {
                toast.error('Unknown error, please try again later.');
            }
        } finally {
            setIsDeleting(false);
        }
    };

    const beginPasskeyReauth = async () => {
        try {
            setReauthStatus('in-progress');
            const token = await beginAccountDeletionReauth();
            setReauthToken(token);
            setReauthStatus('verified');
            toast.success('Identity verified via passkey');
        } catch (err) {
            const error = err as { message?: string };
            const message = error?.message || 'Passkey verification failed';
            setReauthStatus('error');
            toast.error(message);
        }
    };

    const renderVerification = () => {
        return (
            <div className="flex flex-col sm:flex-row sm:items-center gap-3">
                <div className="inline-flex text-[11px] rounded-md bg-[var(--tailwind-colors-slate-800)] p-1 shadow-sm w-full sm:w-auto">
                    <Button
                        type="button"
                        variant={method === 'password' ? 'default' : 'ghost'}
                        size="sm"
                        onClick={() => switchMethod('password')}
                        disabled={!passwordAvailable}
                        className={cn(
                            'flex-1 px-4 py-2 rounded-sm',
                            method === 'password'
                                ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-600)]'
                                : 'text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]',
                            !passwordAvailable && 'opacity-50 cursor-not-allowed'
                        )}
                    >
                        Password
                    </Button>
                    <Button
                        type="button"
                        variant={method === 'passkey' ? 'default' : 'ghost'}
                        size="sm"
                        onClick={() => switchMethod('passkey')}
                        disabled={!hasPasskeys}
                        className={cn(
                            'flex-1 px-4 py-2 rounded-sm',
                            method === 'passkey'
                                ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-600)]'
                                : 'text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]',
                            !hasPasskeys && 'opacity-50 cursor-not-allowed'
                        )}
                    >
                        Passkey
                    </Button>
                </div>
                <span className="text-[11px] text-[var(--tailwind-colors-slate-400)]">Choose verification method</span>
            </div>
        );
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
            setCurrentPassword('');
            setOtp('');
            setReauthToken(null);
            setReauthStatus('idle');
            setHasAttemptedGeneration(false);
            setIsGenerating(false);
            resetMethod();
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
                    {renderVerification()}

                    {method === 'password' && (
                        <div className="space-y-2">
                            <Label htmlFor="current-password" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                Current password
                            </Label>
                            <Input
                                id="current-password"
                                type="password"
                                value={currentPassword}
                                onChange={e => setCurrentPassword(e.target.value)}
                                placeholder="••••••••"
                                className="bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)]"
                            />
                        </div>
                    )}

                    {method === 'passkey' && (
                        <div className="space-y-5">
                            <Label className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                Passkey verification
                            </Label>
                            <div className="flex items-center gap-3">
                                <Button
                                    type="button"
                                    variant={reauthStatus === 'verified' ? 'default' : 'cancel'}
                                    size="lg"
                                    className={cn(
                                        'flex-1 min-h-11 font-medium transition-colors',
                                        reauthStatus === 'verified'
                                            ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]'
                                            : 'border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-rdns-600)] hover:border-[var(--tailwind-colors-slate-400)]'
                                    )}
                                    onClick={beginPasskeyReauth}
                                    disabled={reauthStatus === 'in-progress' || reauthStatus === 'verified'}
                                >
                                    {reauthStatus === 'idle' && 'Verify with passkey'}
                                    {reauthStatus === 'in-progress' && 'Verifying...'}
                                    {reauthStatus === 'verified' && 'Passkey verified'}
                                    {reauthStatus === 'error' && 'Retry passkey'}
                                </Button>
                            </div>
                            {reauthStatus !== 'verified' && (
                                <p className="text-xs text-[var(--tailwind-colors-slate-400)]">Authenticate with a stored passkey to confirm identity.</p>
                            )}
                        </div>
                    )}

                    {showOtp && (
                        <div className="space-y-2">
                            <Label htmlFor="otp" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                2FA code
                            </Label>
                            <Input
                                id="otp"
                                type="text"
                                value={otp}
                                onChange={e => setOtp(e.target.value)}
                                placeholder="6-digit code"
                                className="bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)]"
                            />
                            <p className="text-xs text-[var(--tailwind-colors-slate-400)]">Required for password verification when 2FA is enabled.</p>
                        </div>
                    )}

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
                            (method === 'password' && (!currentPassword.trim() || (showOtp && !otp.trim()))) ||
                            (method === 'passkey' && reauthStatus !== 'verified')
                        }
                    >
                        {isDeleting ? 'Deleting...' : method === 'passkey' && reauthStatus !== 'verified' ? 'Verify passkey first' : 'Delete account'}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
