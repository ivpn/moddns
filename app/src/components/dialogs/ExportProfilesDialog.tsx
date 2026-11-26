import { useState, useEffect, useCallback } from 'react';
import { AlertTriangle, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { DialogBody, DialogActions } from '@/components/dialogs/DialogLayout';
import { MultiSelectProfileList } from '@/components/MultiSelectProfileList';
import { useAppStore } from '@/store/general';
import VerifyIdentityFields, { type IdentityVerification } from '@/components/dialogs/VerifyIdentityFields';
import { useProfileExport } from '@/hooks/useProfileExport';
import { beginProfileExportReauth } from '@/lib/webauthn';
import { cn } from '@/lib/utils';

export interface ExportProfilesDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

type Scope = 'all' | 'selected';

export default function ExportProfilesDialog({ open, onOpenChange }: ExportProfilesDialogProps) {
    const account = useAppStore(s => s.account);
    const storeProfiles = useAppStore(s => s.profiles);

    const profiles = storeProfiles.map(p => ({
        id: p.profile_id,
        name: p.name,
    }));

    const [scope, setScope] = useState<Scope>('all');
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const onVerificationChange = useCallback((v: IdentityVerification | null) => setVerification(v), []);

    const { exportProfiles, isExporting } = useProfileExport();

    useEffect(() => {
        if (!open) {
            setScope('all');
            setSelectedIds([]);
        }
    }, [open]);

    const handleOpenChange = (next: boolean) => {
        if (!next) {
            setScope('all');
            setSelectedIds([]);
        }
        onOpenChange(next);
    };

    const isScopeValid = scope === 'all' || selectedIds.length > 0;

    const isSubmitDisabled = !verification || !isScopeValid || isExporting;

    const handleSubmit = async () => {
        if (isSubmitDisabled || !verification) return;

        const input = {
            scope,
            profileIds: scope === 'selected' ? selectedIds : undefined,
            currentPassword: verification.currentPassword,
            reauthToken: verification.reauthToken,
            otp: verification.otp,
        };

        try {
            await exportProfiles(input);
            const count = scope === 'selected' ? selectedIds.length : profiles.length;
            toast.success(`Exported ${count} profile${count === 1 ? '' : 's'}`);
            handleOpenChange(false);
        } catch {
            // The hook surfaces the failure (including a wrong-password 400) as a
            // toast; keep the dialog open so the user can correct and retry.
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
                <DialogHeader className="px-6 pt-6 pb-3 space-y-1.5">
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
                        <AlertTriangle className="h-4 w-4 text-yellow-600 dark:text-yellow-400 mt-0.5 shrink-0" />
                        <p className="text-sm text-yellow-800 dark:text-yellow-200">
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

                        <VerifyIdentityFields
                            account={account ?? null}
                            open={open}
                            beginPasskeyReauth={beginProfileExportReauth}
                            onChange={onVerificationChange}
                            onSubmit={() => void handleSubmit()}
                            codeTestId="export-code"
                            disabled={isExporting}
                        />
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
