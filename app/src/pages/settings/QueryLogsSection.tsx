import React, { useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { DataCollectionControl } from "@/components/data-collection/DataCollectionControl";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";

interface QueryLogsSectionProps {
    activeProfile: ModelProfile;
}

const QueryLogsSection: React.FC<QueryLogsSectionProps> = ({ activeProfile }) => {
    const headingId = useId();
    const [showClearDialog, setShowClearDialog] = useState(false);
    const [clearLoading, setClearLoading] = useState(false);

    const handleClearLogs = async () => {
        setClearLoading(true);
        try {
            await api.Client.queryLogsApi.apiV1ProfilesIdLogsDelete(activeProfile.profile_id);
            toast.success("Query logs cleared.");
            setShowClearDialog(false);
        } catch {
            toast.error("Failed to clear query logs.");
        } finally {
            setClearLoading(false);
        }
    };

    return (
        <Card className="w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
            <CardContent>
                <div className="flex flex-col items-start gap-6 w-full">
                    <div className="flex items-center gap-2 w-full">
                        <div className="flex flex-col items-start gap-2">
                            <h2 id={headingId} className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                                DATA COLLECTION
                            </h2>
                        </div>
                    </div>

                    <p className="text-sm leading-5 text-[var(--tailwind-colors-slate-200)]">
                        Choose what modDNS keeps about this profile's DNS queries. Off by default.
                    </p>

                    <DataCollectionControl profile={activeProfile} labelledBy={headingId} />

                    {/* Download (GET) and Clear (DELETE on logs) are allowed in limited access at API level */}
                    <div className="flex flex-col sm:flex-row sm:items-center gap-4 w-full">
                        <Button
                            variant="outline"
                            className="px-4 py-2 rounded bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-slate-600)] text-white w-full sm:w-auto"
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
                                } catch {
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