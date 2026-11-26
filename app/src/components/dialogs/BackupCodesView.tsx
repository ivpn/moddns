import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { formatBackupCode } from '@/lib/backupCodes';

interface BackupCodesViewProps {
    codes: string[];
    onDone: () => void;
}

/** A new set of backup codes, shown once, with copy and a Done button. */
export default function BackupCodesView({ codes, onDone }: BackupCodesViewProps) {
    const copyAll = async () => {
        try {
            await navigator.clipboard.writeText(codes.map(formatBackupCode).join('\n'));
            toast.success('Backup codes copied to clipboard');
        } catch {
            toast.error('Failed to copy backup codes');
        }
    };

    return (
        <div className="flex flex-col gap-4 w-full">
            <h2 className="text-base font-semibold text-[var(--tailwind-colors-slate-50)]">Backup codes</h2>
            <p className="text-sm text-[var(--tailwind-colors-slate-200)]">
                Save these backup codes in a safe place. Each code can be used once if you lose access to your authenticator app. They are shown only now.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 bg-[var(--tailwind-colors-slate-900)] p-3 sm:p-4 rounded">
                {codes.map(code => (
                    <span
                        key={code}
                        data-testid="backup-code"
                        className="font-mono text-[var(--tailwind-colors-slate-50)] text-[length:min(1rem,calc((100vw_-_6.5rem)/18))] sm:text-sm px-2 py-1 bg-[var(--tailwind-colors-slate-800)] rounded whitespace-nowrap select-all"
                    >
                        {formatBackupCode(code)}
                    </span>
                ))}
            </div>
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-end mt-4 w-full gap-3">
                <Button
                    variant="outline"
                    size="lg"
                    className="flex-1 sm:flex-none min-w-32 bg-[var(--tailwind-colors-slate-800)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-rdns-600)]"
                    onClick={copyAll}
                >
                    Copy all codes
                </Button>
                <Button
                    size="lg"
                    className="flex-1 sm:flex-none min-w-32 bg-[var(--tailwind-colors-rdns-600)]"
                    onClick={onDone}
                >
                    Done
                </Button>
            </div>
        </div>
    );
}
