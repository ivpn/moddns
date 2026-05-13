import { useState, useEffect, useCallback } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { DialogBody, DialogActions } from '@/components/dialogs/DialogLayout';
import FileDropzone from '@/components/FileDropzone';
import { MultiSelectProfileList } from '@/components/MultiSelectProfileList';
import { useAppStore } from '@/store/general';
import { useAccountVerificationMethod } from '@/hooks/useAccountVerificationMethod';
import { useProfileImport } from '@/hooks/useProfileImport';
import { beginProfileImportReauth } from '@/lib/webauthn';
import { cn } from '@/lib/utils';
import API from '@/api/api';
import type { RequestsImportPayload } from '@/api/client/api';

export interface ImportProfilesDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

type Step = 'pick' | 'confirm' | 'results';
type ReauthStatus = 'idle' | 'in-progress' | 'verified' | 'error';

const MAX_PROFILES = 100;

export default function ImportProfilesDialog({ open, onOpenChange }: ImportProfilesDialogProps) {
    const account = useAppStore(s => s.account);
    const currentProfileCount = useAppStore(s => s.profiles.length);
    const setProfiles = useAppStore(s => s.setProfiles);
    const restoreActiveProfile = useAppStore(s => s.restoreActiveProfile);

    const [step, setStep] = useState<Step>('pick');
    const [parsedPayload, setParsedPayload] = useState<RequestsImportPayload | null>(null);
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [fileError, setFileError] = useState<string | null>(null);
    const [importError, setImportError] = useState<string | null>(null);

    const [currentPassword, setCurrentPassword] = useState('');
    const [otp, setOtp] = useState('');
    const [reauthToken, setReauthToken] = useState<string | null>(null);
    const [reauthStatus, setReauthStatus] = useState<ReauthStatus>('idle');

    const { importProfiles, isImporting, result, reset: resetHook } = useProfileImport();

    const handleMethodChange = useCallback(() => {
        setCurrentPassword('');
        setOtp('');
        setReauthToken(null);
        setReauthStatus('idle');
        setImportError(null);
    }, []);

    const {
        method,
        hasPasskeys,
        passwordAvailable,
        showOtp,
        switchMethod,
        resetMethod,
    } = useAccountVerificationMethod({ account: account ?? null, open, onMethodChange: handleMethodChange });

    const resetAllState = useCallback(() => {
        setStep('pick');
        setParsedPayload(null);
        setSelectedIds([]);
        setFileError(null);
        setImportError(null);
        setCurrentPassword('');
        setOtp('');
        setReauthToken(null);
        setReauthStatus('idle');
        resetMethod();
        resetHook();
    }, [resetMethod, resetHook]);

    useEffect(() => {
        if (!open) {
            resetAllState();
        }
    }, [open, resetAllState]);

    const handleOpenChange = (next: boolean) => {
        if (!next) {
            resetAllState();
        }
        onOpenChange(next);
    };

    // ---------------------------------------------------------------------------
    // Step 1 — file parsing
    // ---------------------------------------------------------------------------

    const handleFileAccepted = (file: File) => {
        setFileError(null);
        const reader = new FileReader();
        reader.onload = () => {
            try {
                const text = reader.result as string;
                const parsed: unknown = JSON.parse(text);
                if (
                    typeof parsed !== 'object' ||
                    parsed === null ||
                    (parsed as Record<string, unknown>)['schemaVersion'] !== 1 ||
                    (parsed as Record<string, unknown>)['kind'] !== 'moddns-export' ||
                    !Array.isArray((parsed as Record<string, unknown>)['profiles']) ||
                    ((parsed as Record<string, unknown>)['profiles'] as unknown[]).length === 0
                ) {
                    setFileError('Invalid export file. Expected a moddns-export JSON with at least one profile.');
                    return;
                }
                const payload = parsed as RequestsImportPayload;
                setParsedPayload(payload);
                setSelectedIds(payload.profiles.map((_, i) => i.toString()));
                setStep('confirm');
            } catch {
                setFileError('Could not parse the file. Make sure it is a valid moddns export JSON.');
            }
        };
        reader.onerror = () => {
            setFileError('Could not read the file. Please try again.');
        };
        reader.readAsText(file);
    };

    const handleFileRejected = (reason: string) => {
        setFileError(reason);
    };

    // ---------------------------------------------------------------------------
    // Step 2 — reauth + submit
    // ---------------------------------------------------------------------------

    const selectedIndices = selectedIds.map(id => parseInt(id, 10));
    const selectedCount = selectedIndices.length;
    const wouldExceedCap = currentProfileCount + selectedCount > MAX_PROFILES;
    const remaining = MAX_PROFILES - currentProfileCount;

    const isReauthComplete =
        method === 'passkey'
            ? reauthStatus === 'verified'
            : currentPassword.trim().length > 0 && (!showOtp || otp.trim().length > 0);

    const isSubmitDisabled =
        selectedCount === 0 ||
        wouldExceedCap ||
        !isReauthComplete ||
        isImporting;

    const beginPasskeyReauth = async () => {
        setReauthStatus('in-progress');
        setImportError(null);
        try {
            const token = await beginProfileImportReauth();
            setReauthToken(token);
            setReauthStatus('verified');
            setCurrentPassword('');
            toast.success('Identity verified via passkey');
        } catch (err) {
            const error = err as { message?: string };
            const msg = error?.message ?? 'Passkey verification failed';
            setReauthStatus('error');
            toast.error(msg);
        }
    };

    const handleImport = async () => {
        if (!parsedPayload || isSubmitDisabled) return;

        setImportError(null);

        const filteredProfiles = selectedIndices
            .filter(i => i >= 0 && i < parsedPayload.profiles.length)
            .map(i => parsedPayload.profiles[i]);

        const filteredPayload: RequestsImportPayload = {
            ...parsedPayload,
            profiles: filteredProfiles,
        };

        try {
            await importProfiles({
                payload: filteredPayload,
                currentPassword: method === 'password' ? currentPassword : undefined,
                reauthToken: method === 'passkey' ? (reauthToken ?? undefined) : undefined,
                // OTP is relevant only for the password method when 2FA is enabled
                otp: method === 'password' && showOtp ? otp : undefined,
            });
            setStep('results');
        } catch (err) {
            const axiosError = err as { response?: { status?: number; data?: { error?: string } } };
            const status = axiosError.response?.status;
            if (status === 401) {
                setImportError('Reauthentication failed. Try again.');
            } else if (status === 400 || status === 413 || status === 415) {
                const serverMsg = axiosError.response?.data?.error ?? 'Import file format is invalid.';
                setImportError(serverMsg);
            } else {
                // Hook already toasted for other errors; keep dialog open with a generic message
                setImportError('Import failed. Check the error above and try again.');
            }
        }
    };

    // ---------------------------------------------------------------------------
    // Step 3 — results
    // ---------------------------------------------------------------------------

    const handleDone = async () => {
        const importedCount = result?.createdProfileIds.length ?? 0;
        const warningCount = result?.warnings.length ?? 0;

        try {
            const response = await API.Client.profilesApi.apiV1ProfilesGet();
            const refreshed = response.data;
            setProfiles(refreshed);
            restoreActiveProfile(refreshed);
        } catch {
            // Non-fatal: store will be refreshed on next navigation
        }

        if (warningCount > 0) {
            toast.warning(`Imported with ${warningCount} warning${warningCount === 1 ? '' : 's'} — review the results`);
        } else {
            toast.success(`Imported ${importedCount} profile${importedCount === 1 ? '' : 's'}`);
        }

        handleOpenChange(false);
    };

    // ---------------------------------------------------------------------------
    // Derived profile list for MultiSelectProfileList (Step 2)
    // ---------------------------------------------------------------------------

    const profileListItems =
        parsedPayload?.profiles.map((p, i) => ({
            id: i.toString(),
            name: p.name,
        })) ?? [];

    // ---------------------------------------------------------------------------
    // Warning row helpers (Step 3)
    // ---------------------------------------------------------------------------

    const isIdnWarning = (warning: string): boolean =>
        warning.includes('internationalized domain');

    // ---------------------------------------------------------------------------
    // Reauth section (shared between Step 2 rendering)
    // ---------------------------------------------------------------------------

    const renderReauthSection = () => (
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
                <span className="text-[11px] text-[var(--tailwind-colors-slate-400)]">
                    Choose verification method
                </span>
            </div>

            {method === 'password' && (
                <div className="space-y-2">
                    <Label
                        htmlFor="import-current-password"
                        className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]"
                    >
                        Current password
                    </Label>
                    <Input
                        id="import-current-password"
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
                            onClick={() => void beginPasskeyReauth()}
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
                    <Label
                        htmlFor="import-otp"
                        className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]"
                    >
                        2FA code
                    </Label>
                    <Input
                        id="import-otp"
                        type="text"
                        value={otp}
                        onChange={e => setOtp(e.target.value)}
                        onKeyDown={e => {
                            if (e.key === 'Enter' && !isSubmitDisabled) {
                                void handleImport();
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

            {importError !== null && (
                <p data-testid="reauth-error" className="text-sm text-[var(--tailwind-colors-red-400)]" role="alert">
                    {importError}
                </p>
            )}
        </div>
    );

    // ---------------------------------------------------------------------------
    // Render
    // ---------------------------------------------------------------------------

    const dialogTitleText =
        step === 'pick'
            ? 'Import profiles'
            : step === 'confirm'
              ? 'Select profiles to import'
              : `Import complete — ${result?.createdProfileIds.length ?? 0} profile${(result?.createdProfileIds.length ?? 0) === 1 ? '' : 's'} restored`;

    const dialogDescriptionText =
        step === 'pick'
            ? 'Select an export file (.moddns.json).'
            : step === 'confirm'
              ? 'Choose which profiles to import and verify your identity.'
              : 'The following profiles were created in your account.';

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent
                data-testid="import-dialog"
                className={cn(
                    'w-full max-w-[calc(100vw-2rem)] sm:max-w-[520px]',
                    'border-[var(--tailwind-colors-slate-600)] p-0',
                    'transition-opacity duration-200',
                    '[&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]'
                )}
            >
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        {dialogTitleText}
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        {dialogDescriptionText}
                    </DialogDescription>
                </DialogHeader>

                {/* ============================================================
                    Step 1 — File pick
                ============================================================ */}
                {step === 'pick' && (
                    <>
                        <DialogBody className="space-y-4">
                            <FileDropzone
                                data-testid="import-dropzone"
                                accept=".moddns.json,.json,application/json"
                                maxBytes={1024 * 1024}
                                onFileAccepted={handleFileAccepted}
                                onFileRejected={handleFileRejected}
                                label="Drop a .moddns.json file here, or click to browse"
                            />
                            {fileError !== null && (
                                <p className="text-sm text-[var(--tailwind-colors-red-400)]" role="alert">
                                    {fileError}
                                </p>
                            )}
                        </DialogBody>
                        <DialogActions>
                            <Button
                                variant="cancel"
                                size="lg"
                                className="flex-1 min-w-32 font-medium"
                                onClick={() => handleOpenChange(false)}
                            >
                                Cancel
                            </Button>
                        </DialogActions>
                    </>
                )}

                {/* ============================================================
                    Step 2 — Select & confirm + reauth
                ============================================================ */}
                {step === 'confirm' && parsedPayload !== null && (
                    <>
                        <DialogBody className="space-y-6">
                            {/* Section: Profile selection */}
                            <div className="space-y-3">
                                <p className="font-mono font-bold text-[var(--tailwind-colors-rdns-600)] uppercase tracking-wider text-sm">
                                    Profile selection
                                </p>
                                <MultiSelectProfileList
                                    profiles={profileListItems}
                                    selectedIds={selectedIds}
                                    onChange={setSelectedIds}
                                    emptyText="No profiles found in the export file."
                                />
                            </div>

                            {/* Cap warning */}
                            {wouldExceedCap && (
                                <div
                                    className="flex items-start gap-3 rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-4 py-3"
                                    role="alert"
                                >
                                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-yellow-400" />
                                    <p className="text-sm text-yellow-200">
                                        Selecting {selectedCount} profile{selectedCount === 1 ? '' : 's'} would exceed the{' '}
                                        {MAX_PROFILES}-profile limit. You have room for {remaining > 0 ? remaining : 0} more.
                                        Deselect some profiles to continue.
                                    </p>
                                </div>
                            )}

                            {/* Reauth */}
                            {renderReauthSection()}
                        </DialogBody>

                        <DialogActions>
                            <Button
                                variant="cancel"
                                size="lg"
                                className="flex-1 min-w-32 font-medium"
                                onClick={() => {
                                    setParsedPayload(null);
                                    setSelectedIds([]);
                                    setFileError(null);
                                    setImportError(null);
                                    setStep('pick');
                                }}
                                disabled={isImporting}
                            >
                                Back
                            </Button>
                            <Button
                                variant="cancel"
                                size="lg"
                                className="flex-1 min-w-32 font-medium"
                                onClick={() => handleOpenChange(false)}
                                disabled={isImporting}
                            >
                                Cancel
                            </Button>
                            <Button
                                data-testid="import-submit-btn"
                                variant="default"
                                size="lg"
                                className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                                onClick={() => void handleImport()}
                                disabled={isSubmitDisabled}
                            >
                                {isImporting ? (
                                    <span className="flex items-center gap-2">
                                        <Loader2 className="animate-spin" />
                                        Importing...
                                    </span>
                                ) : (
                                    'Import profiles'
                                )}
                            </Button>
                        </DialogActions>
                    </>
                )}

                {/* ============================================================
                    Step 3 — Results
                ============================================================ */}
                {step === 'results' && result !== null && (
                    <>
                        <DialogBody data-testid="import-results-step" className="space-y-6">
                            {/* Created profiles list */}
                            <div className="space-y-3">
                                <p className="font-mono font-bold text-[var(--tailwind-colors-rdns-600)] uppercase tracking-wider text-sm">
                                    Created profiles
                                </p>
                                {result.createdProfileIds.length === 0 ? (
                                    <p className="text-sm text-[var(--tailwind-colors-slate-400)]">
                                        No profiles were created.
                                    </p>
                                ) : (
                                    <ul className="flex flex-col gap-1.5">
                                        {result.createdProfileIds.map(id => {
                                            // Attempt to match the created ID back to a name from the
                                            // parsed payload. The backend assigns new IDs, so we can
                                            // only show IDs directly — names are not returned.
                                            return (
                                                <li
                                                    key={id}
                                                    className="flex items-center gap-2 rounded-md bg-[var(--tailwind-colors-slate-800)] px-3 py-2 text-sm"
                                                >
                                                    <span className="font-mono text-[var(--tailwind-colors-rdns-600)] text-xs">
                                                        ID:
                                                    </span>
                                                    <span className="font-mono text-[var(--tailwind-colors-slate-200)] truncate">
                                                        {id}
                                                    </span>
                                                </li>
                                            );
                                        })}
                                    </ul>
                                )}
                            </div>

                            {/* Warnings section */}
                            {result.warnings.length > 0 && (
                                <div className="space-y-3">
                                    <p className="font-mono font-bold text-[var(--tailwind-colors-rdns-600)] uppercase tracking-wider text-sm">
                                        Warnings
                                    </p>
                                    <ul className="flex flex-col gap-2">
                                        {result.warnings.map((warning, idx) => {
                                            const idn = isIdnWarning(warning);
                                            return (
                                                <li
                                                    key={idx}
                                                    data-testid={idn ? 'idn-warning-row' : 'generic-warning-row'}
                                                    className={cn(
                                                        'flex items-start gap-3 rounded-lg border px-4 py-3',
                                                        idn
                                                            ? 'border-yellow-500/30 bg-yellow-500/10'
                                                            : 'border-[var(--tailwind-colors-slate-600)] bg-[var(--tailwind-colors-slate-800)]'
                                                    )}
                                                >
                                                    <AlertTriangle
                                                        className={cn(
                                                            'mt-0.5 h-4 w-4 shrink-0',
                                                            idn
                                                                ? 'text-yellow-400'
                                                                : 'text-[var(--tailwind-colors-slate-400)]'
                                                        )}
                                                    />
                                                    <p
                                                        className={cn(
                                                            'text-sm break-words min-w-0',
                                                            idn
                                                                ? 'text-yellow-200'
                                                                : 'text-[var(--tailwind-colors-slate-300)]'
                                                        )}
                                                    >
                                                        {warning}
                                                    </p>
                                                </li>
                                            );
                                        })}
                                    </ul>
                                </div>
                            )}
                        </DialogBody>

                        <DialogActions>
                            <Button
                                data-testid="import-done-btn"
                                variant="default"
                                size="lg"
                                className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                                onClick={() => void handleDone()}
                            >
                                Done
                            </Button>
                        </DialogActions>
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
