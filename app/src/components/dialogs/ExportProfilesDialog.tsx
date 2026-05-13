import { useState, useEffect, useCallback } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { DialogBody, DialogActions } from '@/components/dialogs/DialogLayout';
import { MultiSelectProfileList } from '@/components/MultiSelectProfileList';
import { useAppStore } from '@/store/general';
import { useAccountVerificationMethod } from '@/hooks/useAccountVerificationMethod';
import { useProfileExport } from '@/hooks/useProfileExport';
import { beginProfileExportReauth } from '@/lib/webauthn';
import { cn } from '@/lib/utils';

export interface ExportProfilesDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

type Scope = 'all' | 'selected';
type ReauthStatus = 'idle' | 'in-progress' | 'verified' | 'error';

export default function ExportProfilesDialog({ open, onOpenChange }: ExportProfilesDialogProps) {
    const account = useAppStore(s => s.account);
    const storeProfiles = useAppStore(s => s.profiles);

    const profiles = storeProfiles.map(p => ({
        id: p.profile_id,
        name: p.name,
    }));

    const [scope, setScope] = useState<Scope>('all');
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [currentPassword, setCurrentPassword] = useState('');
    const [otp, setOtp] = useState('');
    const [reauthToken, setReauthToken] = useState<string | null>(null);
    const [reauthStatus, setReauthStatus] = useState<ReauthStatus>('idle');
    const [reauthError, setReauthError] = useState<string | null>(null);

    const { exportProfiles, isExporting } = useProfileExport();

    const handleMethodChange = useCallback(() => {
        setCurrentPassword('');
        setOtp('');
        setReauthToken(null);
        setReauthStatus('idle');
        setReauthError(null);
    }, []);

    const {
        method,
        hasPasskeys,
        passwordAvailable,
        showOtp,
        switchMethod,
        resetMethod,
    } = useAccountVerificationMethod({ account: account ?? null, open, onMethodChange: handleMethodChange });

    useEffect(() => {
        if (!open) {
            setScope('all');
            setSelectedIds([]);
            setCurrentPassword('');
            setOtp('');
            setReauthToken(null);
            setReauthStatus('idle');
            setReauthError(null);
            resetMethod();
        }
    }, [open, resetMethod]);

    const handleOpenChange = (next: boolean) => {
        if (!next) {
            setScope('all');
            setSelectedIds([]);
            setCurrentPassword('');
            setOtp('');
            setReauthToken(null);
            setReauthStatus('idle');
            setReauthError(null);
            resetMethod();
        }
        onOpenChange(next);
    };

    const beginPasskeyReauth = async () => {
        setReauthStatus('in-progress');
        setReauthError(null);
        try {
            const token = await beginProfileExportReauth();
            setReauthToken(token);
            setReauthStatus('verified');
            setCurrentPassword('');
            toast.success('Identity verified via passkey');
        } catch (err) {
            const error = err as { message?: string };
            const msg = error?.message || 'Passkey verification failed';
            setReauthStatus('error');
            toast.error(msg);
        }
    };

    const isReauthComplete =
        method === 'passkey'
            ? reauthStatus === 'verified'
            : currentPassword.trim().length > 0 && (!showOtp || otp.trim().length > 0);

    const isScopeValid = scope === 'all' || selectedIds.length > 0;

    const isSubmitDisabled = !isReauthComplete || !isScopeValid || isExporting;

    const handleSubmit = async () => {
        if (isSubmitDisabled) return;

        setReauthError(null);

        const input = {
            scope,
            profileIds: scope === 'selected' ? selectedIds : undefined,
            currentPassword: method === 'password' ? currentPassword : undefined,
            reauthToken: method === 'passkey' ? (reauthToken ?? undefined) : undefined,
            // OTP is relevant only for the password method when 2FA is enabled
            otp: method === 'password' && showOtp ? otp : undefined,
        };

        try {
            await exportProfiles(input);
            const count = scope === 'selected' ? selectedIds.length : profiles.length;
            toast.success(`Exported ${count} profile${count === 1 ? '' : 's'}`);
            handleOpenChange(false);
        } catch (err) {
            const axiosError = err as { response?: { status?: number } };
            if (axiosError.response?.status === 401) {
                setReauthError('Reauthentication failed. Try again.');
            }
            // All other errors: hook already toasted; keep dialog open
        }
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent
                data-testid="export-dialog"
                className={cn(
                    'w-full max-w-[calc(100vw-2rem)] sm:max-w-[500px]',
                    'border-[var(--tailwind-colors-slate-600)] p-0',
                    'transition-opacity duration-200',
                    '[&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]'
                )}
            >
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Export profiles
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        Download your DNS profiles as a JSON backup file.
                    </DialogDescription>
                </DialogHeader>

                <DialogBody className="space-y-6">
                    {/* Sensitivity warning — replaces the __warning field that was removed
                        from the export envelope so that DisallowUnknownFields() on import
                        never rejects a fresh export. */}
                    <div className="flex items-start gap-3 rounded-md border border-yellow-500/30 bg-yellow-500/10 p-3" data-testid="export-warning">
                        <AlertTriangle className="h-4 w-4 text-yellow-400 mt-0.5 shrink-0" />
                        <p className="text-sm text-yellow-200">
                            This file contains your DNS filtering rules and configuration. Treat it as confidential — anyone who reads it can see your block/allow lists and may be able to infer your browsing patterns.
                        </p>
                    </div>

                    {/* Section 1 — Profile selection */}
                    <div className="space-y-3">
                        <p className="font-mono font-bold text-[var(--tailwind-colors-rdns-600)] uppercase tracking-wider text-sm">
                            Profile selection
                        </p>

                        <div className="inline-flex text-[11px] rounded-md bg-[var(--tailwind-colors-slate-800)] p-1 shadow-sm w-full sm:w-auto">
                            <Button
                                type="button"
                                variant={scope === 'all' ? 'default' : 'ghost'}
                                size="sm"
                                onClick={() => setScope('all')}
                                className={cn(
                                    'flex-1 px-4 py-2 rounded-sm',
                                    scope === 'all'
                                        ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-600)]'
                                        : 'text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]'
                                )}
                            >
                                All profiles
                            </Button>
                            <Button
                                type="button"
                                variant={scope === 'selected' ? 'default' : 'ghost'}
                                size="sm"
                                onClick={() => setScope('selected')}
                                className={cn(
                                    'flex-1 px-4 py-2 rounded-sm',
                                    scope === 'selected'
                                        ? 'bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-600)]'
                                        : 'text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]'
                                )}
                            >
                                Select profiles
                            </Button>
                        </div>

                        {scope === 'selected' && (
                            <MultiSelectProfileList
                                profiles={profiles}
                                selectedIds={selectedIds}
                                onChange={setSelectedIds}
                                emptyText="No profiles found in your account."
                            />
                        )}
                    </div>

                    {/* Section 2 — Reauth */}
                    <div className="space-y-3">
                        <p className="font-mono font-bold text-[var(--tailwind-colors-rdns-600)] uppercase tracking-wider text-sm">
                            Verification
                        </p>

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

                        {method === 'password' && (
                            <div className="space-y-2">
                                <Label htmlFor="export-current-password" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    Current password
                                </Label>
                                <Input
                                    id="export-current-password"
                                    type="password"
                                    value={currentPassword}
                                    onChange={e => setCurrentPassword(e.target.value)}
                                    placeholder="••••••••"
                                    className="bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                                />
                            </div>
                        )}

                        {method === 'passkey' && (
                            <div className="space-y-3">
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
                                    <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                        Authenticate with a stored passkey to confirm identity.
                                    </p>
                                )}
                            </div>
                        )}

                        {showOtp && (
                            <div className="space-y-2">
                                <Label htmlFor="export-otp" className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                                    2FA code
                                </Label>
                                <Input
                                    id="export-otp"
                                    type="text"
                                    value={otp}
                                    onChange={e => setOtp(e.target.value)}
                                    onKeyDown={e => {
                                        if (e.key === 'Enter' && !isSubmitDisabled) {
                                            void handleSubmit();
                                        }
                                    }}
                                    placeholder="6-digit code"
                                    className="bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] h-10"
                                />
                                <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                    Required for password verification when 2FA is enabled.
                                </p>
                            </div>
                        )}

                        {reauthError !== null && (
                            <p data-testid="reauth-error" className="text-sm text-[var(--tailwind-colors-red-400)]" role="alert">
                                {reauthError}
                            </p>
                        )}
                    </div>
                </DialogBody>

                <DialogActions>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => handleOpenChange(false)}
                        disabled={isExporting}
                    >
                        Cancel
                    </Button>
                    <Button
                        data-testid="export-submit-btn"
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                        onClick={() => void handleSubmit()}
                        disabled={isSubmitDisabled}
                    >
                        {isExporting ? (
                            <span className="flex items-center gap-2">
                                <Loader2 className="animate-spin" />
                                Exporting...
                            </span>
                        ) : (
                            'Export profiles'
                        )}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
