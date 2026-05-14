import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Info } from "lucide-react";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import StatusBadge from "@/components/general/StatusBadge";
import api from "@/api/api";
import type { ModelSubscription } from "@/api/client/api";
import { useAppStore } from "@/store/general";

const RESYNC_URL = import.meta.env.VITE_RESYNC_URL || "https://www.ivpn.net/en/account/";

const UUID_V4_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;
const isUUIDv4 = (id: string): boolean => UUID_V4_RE.test(id);

export default function AccountSubscription() {
    const [sub, setSub] = useState<ModelSubscription | null>(null);
    const [error, setError] = useState("");
    const [syncing, setSyncing] = useState(false);
    const [searchParams] = useSearchParams();
    const setSubscriptionStatus = useAppStore(s => s.setSubscriptionStatus);

    const sessionid = searchParams.get("sessionid") || "";
    const subid = searchParams.get("subid") || "";

    const fetchSubscription = async () => {
        try {
            const res = await api.Client.subscriptionApi.apiV1SubGet();
            setSub(res.data);
            setSubscriptionStatus(res.data.status ?? null);
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
            // Only forward subid when it's a valid UUIDv4; backend skips the
            // signup webhook entirely when subid is absent.
            const body = isUUIDv4(subid) ? { subid } : undefined;
            await api.Client.subscriptionApi.apiV1SubUpdatePut(body);
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
    ];

    // "Active until" is only meaningful while the subscription is still active or in grace —
    // hide it once the user lands in Limited Access or Pending Delete.
    if (!isLimited && !isPendingDelete) {
        rows.push({ label: "Active until", value: formatDate(sub.active_until) });
    }

    return (
        <>
            {/* Alerts — rendered first, meant to be placed above the cards by parent */}
            {hasAlerts && (
                <div className="col-span-full flex flex-col gap-3 order-first">
                    {isLimited && (
                        <div className="flex items-center gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
                            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />
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

                    {sub.outage && (
                        <div className="flex items-center gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
                            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />
                            <div className="min-w-0">
                                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                                    Out of sync
                                </p>
                                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                                    Your last account status update was {sub.updated_at ? formatDate(sub.updated_at) : "unknown"}.{" "}
                                    <a href={`${RESYNC_URL}?action=sync&service=dns`} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">Sync with IVPN</a>
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

                    {/* min-h preserves the card height it had when a third "Last synced" row was rendered:
                        mobile rows stack (label+value, ~44px each) → 3*44 + 2*12 ≈ 156px;
                        desktop rows are single-line (~20px each) → 3*20 + 2*12 ≈ 84px. */}
                    <div className="flex flex-col gap-3 min-h-[9.75rem] sm:min-h-[5.25rem]">
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
