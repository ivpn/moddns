import React, { useId } from "react";
import { useLocation } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { DataCollectionControl } from "@/components/data-collection/DataCollectionControl";
import { useClearQueryLogs } from "@/components/data-collection/ClearQueryLogs";
import { useDeleteStatisticsHistory } from "@/components/data-collection/DeleteStatisticsHistory";
import { DATA_COLLECTION_ANCHOR, STALE_TEXT } from "@/components/data-collection/model";

interface QueryLogsSectionProps {
    activeProfile: ModelProfile;
}

const QueryLogsSection: React.FC<QueryLogsSectionProps> = ({ activeProfile }) => {
    const headingId = useId();
    // S4: /settings#data-collection lands on the statistics retention.
    const deepLinked = useLocation().hash === `#${DATA_COLLECTION_ANCHOR}`;
    const statsOn = !!activeProfile.settings?.statistics?.enabled;
    const history = useDeleteStatisticsHistory(activeProfile);
    const clear = useClearQueryLogs(activeProfile.profile_id);

    return (
        <Card id={DATA_COLLECTION_ANCHOR} className="scroll-mt-6 w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
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

                    <DataCollectionControl
                        profile={activeProfile}
                        labelledBy={headingId}
                        initialFocus={deepLinked ? "stats-retention" : undefined}
                    />

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
                            onClick={clear.request}
                        >
                            <span className="text-white">Clear query logs</span>
                        </Button>
                        {statsOn && (
                            <Button
                                variant="outline"
                                className="h-auto min-h-11 lg:min-h-0 px-2 py-1.5 border-[var(--tailwind-colors-red-600)] text-[var(--tailwind-colors-red-600)] dark:text-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                                disabled={history.busy}
                                onClick={() => void history.request()}
                            >
                                Delete statistics history
                            </Button>
                        )}
                    </div>
                    {history.stale && (
                        <p aria-live="polite" className="text-sm leading-5 text-[var(--tailwind-colors-slate-200)]">
                            {STALE_TEXT}
                        </p>
                    )}
                    {history.dialog}
                </div>
            </CardContent>

            {clear.dialog}
        </Card>
    );
};

export default QueryLogsSection;