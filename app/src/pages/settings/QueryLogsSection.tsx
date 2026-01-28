import React, { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import ToggleGroup from "@/components/general/ToggleGroup";
import { toast } from "sonner";
import api from "@/api/api";
import { Tooltip } from "@/components/ui/tooltip";
import { Info } from "lucide-react";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";

type ToggleOption = { value: string; label: string };
export interface LogsSetting {
    title?: string;
    description?: string;
    value: string;
    options: ToggleOption[];
}
interface ActiveProfile { profile_id: string }
interface QueryLogsSectionProps {
    logsSettings: LogsSetting[];
    activeProfile: ActiveProfile;
    handleLogsChange: (idx: number, value: string) => void;
}

const QueryLogsSection: React.FC<QueryLogsSectionProps> = ({
    logsSettings,
    activeProfile,
    handleLogsChange,
}) => {
    const [showClearDialog, setShowClearDialog] = useState(false);
    const [clearLoading, setClearLoading] = useState(false);

    const handleClearLogs = async () => {
        setClearLoading(true);
        try {
            await api.Client.queryLogsApi.apiV1ProfilesIdLogsDelete(activeProfile.profile_id);
            toast.success("Query logs cleared.");
            setShowClearDialog(false);
        } catch (_err: unknown) {
            toast.error("Failed to clear query logs.");
        } finally {
            setClearLoading(false);
        }
    };

    return (
        <Card className="w-full border-none">
            <CardContent>
                <div className="flex flex-col items-start gap-6 w-full">
                    <div className="flex items-center gap-2 w-full">
                        <div className="flex flex-col items-start gap-2">
                            <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                LOGS
                            </div>
                        </div>
                    </div>

                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full">
                        <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                            <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">Query logs</div>
                            <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                Logs are disabled by default to protect your privacy.
                            </div>
                        </div>
                        <ToggleGroup
                            options={logsSettings[0]?.options || []}
                            value={logsSettings[0]?.value || "disable"}
                            onChange={value => handleLogsChange(0, value)}
                            variant="outline"
                            className="rounded p-0.5 self-start sm:self-auto"
                        />
                    </div>

                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full">
                        <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                            <div className="flex items-center gap-1">
                                <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">Retention period</div>
                                {/* Informational tooltip about retention period reset behavior */}
                                <Tooltip
                                    content={
                                        <span>
                                            Changing the retention period switches to a new set of query logs. Logs collected under your previous setting remain preserved and become accessible again if you revert to that earlier retention period.
                                        </span>
                                    }
                                    side="top"
                                    align="start"
                                    delay={0}
                                    maxWidthClassName="max-w-[260px] md:max-w-[300px]"
                                >
                                    <button
                                        type="button"
                                        aria-label="Retention period information"
                                        data-testid="retention-info-trigger"
                                        className="p-0.5 rounded focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-300)] hover:text-[var(--tailwind-colors-slate-50)] transition-colors"
                                    >
                                        <Info size={16} strokeWidth={2} />
                                    </button>
                                </Tooltip>
                            </div>
                            <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                Choose how long query logs are kept before being automatically deleted.
                            </div>
                        </div>
                        <div className="w-full sm:w-auto md:flex-shrink-0">
                            <ToggleGroup
                                options={logsSettings[3]?.options || []}
                                value={logsSettings[3]?.value || "1d"}
                                onChange={value => handleLogsChange(3, value)}
                                variant="outline"
                                className="!w-full flex flex-wrap md:flex-nowrap gap-1 sm:!w-auto"
                                itemClassName="min-w-[52px] sm:min-w-[64px] md:min-w-[64px] flex-1 sm:flex-initial px-2 sm:px-3"
                            />
                        </div>
                    </div>

                    {logsSettings.slice(1, 3).map((setting, index) => (
                        <div
                            key={index + 1}
                            className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full"
                        >
                            <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                                <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">{setting.title}</div>
                                <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">{setting.description}</div>
                            </div>
                            <ToggleGroup
                                options={setting.options}
                                value={setting.value}
                                onChange={value => handleLogsChange(index + 1, value)}
                                variant="outline"
                                className="rounded p-0.5 self-start sm:self-auto"
                            />
                        </div>
                    ))}

                    <div className="flex flex-col sm:flex-row sm:items-center gap-4 w-full">
                        <Button
                            variant="outline"
                            className="px-4 py-2 rounded bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)] w-full sm:w-auto"
                            onClick={async () => {
                                try {
                                    // Request JSON (array of query logs). Avoid forcing blob response which led to JSON.stringify(Blob) -> '{}'.
                                    const response = await api.Client.queryLogsApi.apiV1ProfilesIdLogsDownloadGet(
                                        activeProfile.profile_id
                                    );

                                    let blob: Blob;
                                    const data = response.data;
                                    // If underlying client already returned a Blob, use it directly; otherwise serialize JSON.
                                    if (data instanceof Blob) {
                                        blob = data;
                                    } else {
                                        if (!data || (Array.isArray(data) && data.length === 0)) {
                                            toast.error("No query logs available to download.");
                                            return;
                                        }
                                        blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
                                    }
                                    // Create a download link
                                    const url = window.URL.createObjectURL(blob);
                                    const link = document.createElement("a");
                                    link.href = url;
                                    // Try to get filename from headers, fallback to default
                                    const disposition = response.headers?.["content-disposition"];
                                    let filename = "query-logs.json";
                                    if (disposition) {
                                        const match = disposition.match(/filename="?([^"]+)"?/);
                                        if (match) filename = match[1];
                                    }
                                    link.setAttribute("download", filename);
                                    document.body.appendChild(link);
                                    link.click();
                                    link.remove();
                                    window.URL.revokeObjectURL(url);
                                    toast.success("Query logs download started.");
                                } catch (_err: unknown) {
                                    toast.error("Failed to download query logs.");
                                }
                            }}
                        >
                            Download query logs
                        </Button>
                        <Button
                            className="h-auto min-h-11 lg:min-h-0 flex items-center justify-center px-2 py-1.5 bg-[var(--tailwind-colors-red-600)] rounded-[var(--primitives-radius-radius-md)] gap-1 hover:bg-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                            onClick={() => setShowClearDialog(true)}
                        >
                            <span className="text-white">Clear query logs</span>
                        </Button>
                    </div>
                </div>
            </CardContent>

            {/* Confirm Clear Logs Dialog */}
            <Dialog open={showClearDialog} onOpenChange={setShowClearDialog}>
                <DialogContent
                    className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] px-4 sm:px-0"
                >
                    <DialogHeader className="p-6 pb-0">
                        <DialogTitle className="text-lg tracking-[-0.45px] leading-[18px] font-semibold text-[var(--tailwind-colors-slate-50)]">
                            Clear all query logs?
                        </DialogTitle>
                    </DialogHeader>
                    <DialogDescription className="px-6 pt-2 pb-0 text-sm tracking-[-0.35px] leading-[19.6px] text-[var(--tailwind-colors-slate-50)]">
                        This action cannot be undone. All query logs for this profile will be permanently deleted.
                    </DialogDescription>
                    <DialogActions>
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium focus:outline-none focus-visible:outline-none focus:ring-0 focus-visible:ring-0 outline-none [-webkit-tap-highlight-color:transparent]"
                            onClick={() => setShowClearDialog(false)}
                            disabled={clearLoading}
                        >
                            Cancel
                        </Button>
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                            onClick={handleClearLogs}
                            disabled={clearLoading}
                        >
                            {clearLoading ? "Clearing..." : "Clear logs"}
                        </Button>
                    </DialogActions>
                </DialogContent>
            </Dialog>
        </Card>
    );
};

export default QueryLogsSection;