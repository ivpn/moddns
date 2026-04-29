import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Info } from "lucide-react";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import StatusBadge from "@/components/general/StatusBadge";
import api from "@/api/api";
import type { ModelSubscription } from "@/api/client/api";

const RESYNC_URL = import.meta.env.VITE_RESYNC_URL || "https://www.ivpn.net/en/account/";

export default function AccountSubscription() {
    const [sub, setSub] = useState<ModelSubscription | null>(null);
    const [error, setError] = useState("");
    const [syncing, setSyncing] = useState(false);
    const [searchParams] = useSearchParams();

    const sessionid = searchParams.get("sessionid") || "";

    const fetchSubscription = async () => {
        try {
            const res = await api.Client.subscriptionApi.apiV1SubGet();
            setSub(res.data);
        } catch {
            setError("Failed to load subscription.");
        }
    };

    const resync = async () => {
        if (!sessionid) return;

        setSyncing(true);
        setError("");
        try {
            await api.Client.paSessionApi.apiV1PasessionRotatePut({ sessionid });
            await api.Client.subscriptionApi.apiV1SubUpdatePut();
            await fetchSubscription();
            toast.success("Your account has been successfully synced.");
        } catch {
            setError("Failed to sync subscription. Please try again.");
        } finally {
            setSyncing(false);
        }
    };

    useEffect(() => { fetchSubscription(); }, []);

    useEffect(() => {
        if (sessionid) { resync(); }
    }, [sessionid]); // eslint-disable-line react-hooks/exhaustive-deps

    if (!sub) return null;

    const isActive = sub.status === "active" || sub.status === "grace_period";
    const isLimited = sub.status === "limited_access";
    const isPendingDelete = sub.status === "pending_delete";
    const hasAlerts = isLimited || isPendingDelete || sub.outage || !!error;

    const statusBadge = syncing
        ? <StatusBadge intent="info" text="Syncing..." />
        : isActive
            ? <StatusBadge intent="success" text="Active" />
            : isLimited
                ? <StatusBadge intent="warning" text="Limited" />
                : isPendingDelete
                    ? <StatusBadge intent="error" text="Pending deletion" />
                    : <StatusBadge intent="neutral" text="Inactive" />;

    const formatDate = (dateStr?: string) => {
        if (!dateStr) return "—";
        return new Date(dateStr).toLocaleDateString(undefined, {
            year: "numeric", month: "short", day: "numeric",
        });
    };

    const rows: { label: string; value: React.ReactNode }[] = [
        { label: "Status", value: statusBadge },
        { label: "Active until", value: formatDate(sub.active_until) },
    ];

    if (sub.updated_at) {
        rows.push({ label: "Last synced", value: formatDate(sub.updated_at) });
    }

    return (
        <>
            {/* Alerts — rendered first, meant to be placed above the cards by parent */}
            {hasAlerts && (
                <div className="col-span-full flex flex-col gap-3 order-first">
                    {isLimited && (
                        <div className="flex items-start gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
                            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] mt-0.5 flex-shrink-0" />
                            <div className="min-w-0">
                                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                                    Limited Access Mode
                                </p>
                                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                                    Your modDNS account is in limited access mode. To regain full access add time to your{" "}
                                    <a href={RESYNC_URL} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">IVPN account</a>.
                                </p>
                            </div>
                        </div>
                    )}

                    {isPendingDelete && (
                        <div className="flex items-start gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
                            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] mt-0.5 flex-shrink-0" />
                            <div className="min-w-0">
                                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                                    Pending Deletion
                                </p>
                                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                                    Your account is pending deletion. To reinstate access add time to your{" "}
                                    <a href={RESYNC_URL} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">IVPN account</a>.
                                </p>
                            </div>
                        </div>
                    )}

                    {sub.outage && (
                        <div className="flex items-start gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
                            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] mt-0.5 flex-shrink-0" />
                            <div className="min-w-0">
                                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                                    Out of sync
                                </p>
                                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                                    Your last account status update was {sub.updated_at ? formatDate(sub.updated_at) : "unknown"}.{" "}
                                    <a href={`${RESYNC_URL}?action=sync`} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">Sync with IVPN</a>
                                </p>
                            </div>
                        </div>
                    )}

                    {error && (
                        <p className="font-['Figtree',Helvetica] text-xs text-[var(--tailwind-colors-red-400)]">{error}</p>
                    )}
                </div>
            )}

            {/* Subscription info card */}
            <Card className="w-full max-w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent overflow-hidden">
                <CardContent className="flex flex-col gap-6 w-full max-w-full">
                    <div className="flex flex-col gap-0.5">
                        <h2 className="font-mono font-bold text-[var(--tailwind-colors-slate-50)] text-xl leading-6">
                            Subscription
                        </h2>
                    </div>

                    <div className="flex flex-col gap-3">
                        {rows.map((item, index) => (
                            <div
                                key={index}
                                className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-1 sm:gap-3 break-words min-w-0"
                            >
                                <span className="font-['Figtree',Helvetica] font-normal text-[var(--tailwind-colors-slate-100)] text-sm leading-[20px] break-words min-w-0">
                                    {item.label}
                                </span>
                                <div className="flex items-center justify-start sm:justify-end gap-2 flex-wrap max-w-full break-words min-w-0">
                                    {typeof item.value === "string" ? (
                                        <span className="text-[var(--tailwind-colors-slate-50)] text-sm leading-5 break-words break-all min-w-0">
                                            {item.value}
                                        </span>
                                    ) : (
                                        item.value
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                </CardContent>
            </Card>
        </>
    );
}
