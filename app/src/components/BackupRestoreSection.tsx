import { useState } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useSubscriptionGuard } from '@/hooks/useSubscriptionGuard';
import ExportProfilesDialog from '@/components/dialogs/ExportProfilesDialog';
import ImportProfilesDialog from '@/components/dialogs/ImportProfilesDialog';

export default function BackupRestoreSection() {
    const { isRestricted } = useSubscriptionGuard();
    const [exportOpen, setExportOpen] = useState(false);
    const [importOpen, setImportOpen] = useState(false);

    return (
        <>
            <Card
                data-testid="backup-restore-section"
                className="w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent"
            >
                <CardContent>
                    <div className="flex flex-col items-start gap-6 w-full">
                        <div className="flex items-center gap-2 w-full">
                            <div className="flex flex-col items-start gap-2">
                                <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                    BACKUP &amp; RESTORE
                                </div>
                            </div>
                        </div>

                        {/* Export row */}
                        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full">
                            <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                                <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">
                                    Export profiles
                                </div>
                                <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                    Download your profile configuration as a JSON file
                                </div>
                            </div>
                            <Button
                                data-testid="btn-export-profiles"
                                className="h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)] w-full sm:w-auto"
                                onClick={() => setExportOpen(true)}
                            >
                                <span className="text-sm break-words">Export profiles</span>
                            </Button>
                        </div>

                        {/* Import row */}
                        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full">
                            <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                                <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">
                                    Import profiles
                                </div>
                                <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                    Restore profiles from a previously exported file
                                </div>
                            </div>
                            <span title={isRestricted ? 'Active subscription required to import' : undefined}>
                                <Button
                                    data-testid="btn-import-profiles"
                                    className={`h-auto min-h-11 lg:min-h-0 bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-800)] hover:text-[var(--tailwind-colors-rdns-600)] w-full sm:w-auto${isRestricted ? ' opacity-50 cursor-not-allowed' : ''}`}
                                    disabled={isRestricted}
                                    onClick={() => setImportOpen(true)}
                                >
                                    <span className="text-sm break-words">Import profiles</span>
                                </Button>
                            </span>
                        </div>
                    </div>
                </CardContent>
            </Card>

            <ExportProfilesDialog open={exportOpen} onOpenChange={setExportOpen} />
            <ImportProfilesDialog open={importOpen} onOpenChange={setImportOpen} />
        </>
    );
}
