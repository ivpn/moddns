import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { ModelAccount } from '@/api/client/api';
import { cn } from '@/lib/utils';
import { useAccountVerificationMethod } from '@/hooks/useAccountVerificationMethod';

/** A completed identity check: the current password (plus a 2FA code when TOTP is on) or a passkey reauth token. */
export interface IdentityVerification {
    currentPassword?: string;
    otp?: string;
    reauthToken?: string;
}

interface VerifyIdentityFieldsProps {
    account: ModelAccount | null;
    open: boolean;
    /** Runs the passkey ceremony for the operation's reauth purpose and returns the reauth token. */
    beginPasskeyReauth: () => Promise<string>;
    /** Called with the verification once it is complete, and with null while it is not. */
    onChange: (verification: IdentityVerification | null) => void;
    /** Offer only passkey verification (e.g. the account has no password yet). */
    passkeyOnly?: boolean;
    disabled?: boolean;
}

const fieldClass = 'border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-50)] h-10';
const toggleClass = (active: boolean, available: boolean) => cn(
    'flex-1 px-4 py-2 rounded-sm',
    active
        ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-600)]'
        : 'text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]',
    !available && 'opacity-50 cursor-not-allowed'
);

export default function VerifyIdentityFields({
    account,
    open,
    beginPasskeyReauth,
    onChange,
    passkeyOnly = false,
    disabled = false,
}: VerifyIdentityFieldsProps) {
    const [currentPassword, setCurrentPassword] = useState('');
    const [otp, setOtp] = useState('');
    const [reauthToken, setReauthToken] = useState<string | null>(null);
    const [reauthStatus, setReauthStatus] = useState<'idle' | 'in-progress' | 'verified' | 'error'>('idle');

    const reset = useCallback(() => {
        setCurrentPassword('');
        setOtp('');
        setReauthToken(null);
        setReauthStatus('idle');
    }, []);

    const { method: chosenMethod, hasPasskeys, passwordAvailable, showOtp: otpForPassword, switchMethod } =
        useAccountVerificationMethod({ account, open, onMethodChange: reset });
    const method = passkeyOnly ? 'passkey' : chosenMethod;
    const showOtp = method === 'password' && otpForPassword;

    useEffect(() => {
        if (!open) reset();
    }, [open, reset]);

    useEffect(() => {
        if (method === 'passkey') {
            onChange(reauthToken ? { reauthToken } : null);
        } else if (currentPassword && (!showOtp || otp)) {
            onChange(showOtp ? { currentPassword, otp } : { currentPassword });
        } else {
            onChange(null);
        }
    }, [method, reauthToken, currentPassword, otp, showOtp, onChange]);

    const verifyWithPasskey = async () => {
        setReauthStatus('in-progress');
        try {
            setReauthToken(await beginPasskeyReauth());
            setReauthStatus('verified');
            toast.success('Identity verified via passkey');
        } catch (e) {
            const err = e as { message?: string };
            setReauthStatus('error');
            toast.error(err?.message || 'Passkey verification failed');
        }
    };

    return (
        <div className="flex flex-col gap-4">
            {!passkeyOnly && (
                <div className="flex flex-col sm:flex-row sm:items-center gap-3">
                    <div className="inline-flex text-[11px] rounded-md bg-[var(--tailwind-colors-slate-800)] p-1 shadow-sm w-full sm:w-auto">
                        <Button type="button" variant={method === 'password' ? 'default' : 'ghost'} size="sm"
                            onClick={() => switchMethod('password')} disabled={!passwordAvailable || disabled}
                            className={toggleClass(method === 'password', passwordAvailable)}>
                            Password
                        </Button>
                        <Button type="button" variant={method === 'passkey' ? 'default' : 'ghost'} size="sm"
                            onClick={() => switchMethod('passkey')} disabled={!hasPasskeys || disabled}
                            className={toggleClass(method === 'passkey', hasPasskeys)}>
                            Passkey
                        </Button>
                    </div>
                    <span className="text-[11px] text-[var(--tailwind-colors-slate-400)]">Choose verification method</span>
                </div>
            )}
            {method === 'password' && (
                <div className="space-y-2">
                    <label htmlFor="verify-current-password" className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">Current password</label>
                    <Input id="verify-current-password" type="password" autoComplete="current-password" value={currentPassword}
                        onChange={e => setCurrentPassword(e.target.value)} disabled={disabled} className={fieldClass} />
                </div>
            )}
            {method === 'passkey' && (
                <div className="space-y-3">
                    <label className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">Passkey verification</label>
                    <Button type="button" variant={reauthStatus === 'verified' ? 'default' : 'cancel'} size="lg"
                        className={cn('min-h-11 w-full font-medium transition-colors',
                            reauthStatus === 'verified'
                                ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]'
                                : 'border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-rdns-600)] hover:border-[var(--tailwind-colors-slate-400)]')}
                        onClick={verifyWithPasskey}
                        disabled={disabled || reauthStatus === 'in-progress' || reauthStatus === 'verified'}>
                        {reauthStatus === 'idle' && 'Verify with passkey'}
                        {reauthStatus === 'in-progress' && 'Verifying...'}
                        {reauthStatus === 'verified' && 'Passkey verified'}
                        {reauthStatus === 'error' && 'Retry passkey'}
                    </Button>
                    {reauthStatus !== 'verified' && (
                        <p className="text-xs text-[var(--tailwind-colors-slate-400)]">Authenticate with a stored passkey to confirm it's you.</p>
                    )}
                </div>
            )}
            {showOtp && (
                <div className="space-y-2">
                    <label htmlFor="verify-otp" className="text-sm text-[var(--tailwind-colors-slate-50)] font-medium">2FA code</label>
                    <Input id="verify-otp" inputMode="numeric" autoComplete="one-time-code" value={otp}
                        onChange={e => setOtp(e.target.value)} disabled={disabled} className={fieldClass} />
                    <p className="text-xs text-[var(--tailwind-colors-slate-400)]">6-digit code</p>
                </div>
            )}
        </div>
    );
}
