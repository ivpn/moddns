import { useState } from 'react';
import { KeyRound, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useAppStore } from '@/store/general';
import RegenerateBackupCodesDialog from '@/pages/account_preferences/RegenerateBackupCodes';

/** Shown while the 2FA backup codes are of an earlier format (api-endpoint-behaviour E14);
 *  closing it hides it until the next sign-in. */
export default function LegacyBackupCodesBanner() {
    const totp = useAppStore(s => s.account?.mfa?.totp);
    const dismissed = useAppStore(s => s.legacyBackupCodesBannerDismissed);
    const setDismissed = useAppStore(s => s.setLegacyBackupCodesBannerDismissed);
    const [regenerating, setRegenerating] = useState(false);

    const legacy = !!totp?.enabled && !!totp.backup_codes_legacy;
    if (!legacy && !regenerating) return null;

    return (
        <>
            {legacy && !dismissed && (
                <div
                    data-testid="legacy-backup-codes-banner"
                    className="flex items-start gap-3 w-full rounded-lg border-2 border-yellow-600 dark:border-yellow-400 bg-yellow-500/10 px-4 py-3"
                >
                    <KeyRound className="w-5 h-5 mt-0.5 flex-shrink-0 text-yellow-600 dark:text-yellow-400" aria-hidden="true" />
                    <div className="min-w-0 flex-1 text-sm leading-5">
                        <p className="font-['Figtree',Helvetica] font-semibold text-[var(--shadcn-ui-app-foreground)]">
                            We strongly advise regenerating your 2FA backup codes.
                        </p>
                        <p className="mt-0.5 font-['Figtree',Helvetica] text-[var(--shadcn-ui-app-foreground)]">
                            Your backup codes were created before we improved how backup codes are generated and stored. They still work, but a new set gives your account stronger protection. It takes a minute, and your authenticator app keeps working as before.
                        </p>
                        <Button
                            type="button"
                            size="sm"
                            data-testid="legacy-backup-codes-banner-regenerate"
                            onClick={() => setRegenerating(true)}
                            className="mt-3 h-auto min-h-11 lg:min-h-8 w-full sm:w-auto bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)]"
                        >
                            <span className="text-sm break-words">Regenerate backup codes</span>
                        </Button>
                    </div>
                    <button
                        type="button"
                        aria-label="Close"
                        onClick={() => setDismissed(true)}
                        data-testid="legacy-backup-codes-banner-close"
                        className="flex-shrink-0 -mr-2 -my-1.5 flex h-8 w-8 items-center justify-center rounded-md text-[var(--shadcn-ui-app-foreground)] hover:bg-yellow-500/20 cursor-pointer"
                    >
                        <X className="w-4 h-4" aria-hidden="true" />
                    </button>
                </div>
            )}
            {/* Stays mounted while the new codes are shown, after the banner itself is gone. */}
            {regenerating && <RegenerateBackupCodesDialog open={regenerating} onOpenChange={setRegenerating} />}
        </>
    );
}
