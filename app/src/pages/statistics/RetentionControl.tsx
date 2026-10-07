// "Kept for" select and the "Delete statistics history" menu (#713).

import { useId, useRef, useState } from "react";
import { EllipsisVertical, Trash2 } from "lucide-react";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import {
    STATS_RETENTION_OPTIONS,
    profileStatsRetention,
    statsRetentionWords,
    type StatsRetention,
} from "@/components/data-collection/model";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import { useAppStore } from "@/store/general";
import { cn } from "@/lib/utils";
import { deleteStatisticsHistory, setStatisticsRetention } from "./retentionApi";
import { LimitedAccessNote, mutedText, titleText } from "./primitives";

type Pending = { kind: "raise" | "lower"; to: StatsRetention } | { kind: "delete" };

const days = (r: StatsRetention) => STATS_RETENTION_OPTIONS.find(o => o.value === r)!.days;

function dialogCopy(p: Pending): { title: string; body: string; confirm: string; busy: string; red: boolean } {
    if (p.kind === "delete") {
        return {
            title: "Delete statistics history?",
            body: "All statistics for this profile will be permanently deleted. Statistics stay on and counting starts again now. This action cannot be undone.",
            confirm: "Delete history",
            busy: "Deleting…",
            red: true,
        };
    }
    const words = statsRetentionWords(p.to);
    return p.kind === "raise"
        ? {
              title: `Keep statistics for ${words}?`,
              body: `modDNS will keep this profile's query counts per device for up to ${words}. No domains or addresses are stored. You can lower this or delete the history at any time.`,
              confirm: `Keep for ${words}`,
              busy: "Saving…",
              red: false,
          }
        : {
              title: `Keep statistics for ${words}?`,
              body: `Counts older than ${words} will be permanently deleted. This action cannot be undone.`,
              confirm: "Delete older counts",
              busy: "Deleting…",
              red: true,
          };
}

export function RetentionControl({ profile, onHistoryDeleted }: { profile: ModelProfile; onHistoryDeleted: () => void }) {
    const uid = useId();
    const { isRestricted } = useSubscriptionGuard();
    const setActiveProfile = useAppStore(s => s.setActiveProfile);
    const setProfiles = useAppStore(s => s.setProfiles);
    const saved = profileStatsRetention(profile);
    const [pending, setPending] = useState<Pending | null>(null);
    const [busy, setBusy] = useState(false);
    const selectRef = useRef<HTMLSelectElement>(null);
    const menuRef = useRef<HTMLButtonElement>(null);
    const noteId = `${uid}-la`;

    const shown = pending && pending.kind !== "delete" ? pending.to : saved;

    const writeProfile = (p: ModelProfile) => {
        setActiveProfile(p);
        setProfiles(useAppStore.getState().profiles.map(x => (x.profile_id === p.profile_id ? p : x)));
    };

    const onSelect = (value: string) => {
        const to = value as StatsRetention;
        if (to === saved) return;
        setPending({ kind: days(to) > days(saved) ? "raise" : "lower", to });
    };

    const close = (restoreFocus: "select" | "menu") => {
        setPending(null);
        requestAnimationFrame(() => (restoreFocus === "select" ? selectRef.current : menuRef.current)?.focus());
    };

    const confirm = async () => {
        if (!pending) return;
        setBusy(true);
        const p = pending;
        try {
            if (p.kind === "delete") {
                await deleteStatisticsHistory(profile.profile_id);
                try {
                    writeProfile((await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id)).data);
                } catch {
                    // The statistics refetch below carries history_deleted_at as well.
                }
                toast.success("Statistics history deleted.");
                setPending(null);
                onHistoryDeleted();
            } else {
                writeProfile(await setStatisticsRetention(profile.profile_id, p.to));
                const words = statsRetentionWords(p.to);
                toast.success(
                    p.kind === "raise" ? `Statistics are now kept for ${words}.` : `Statistics are now kept for ${words}. Older counts deleted.`,
                );
                close("select");
            }
        } catch (e) {
            // A 5xx can follow a committed write, so show whatever the server holds now.
            try {
                writeProfile((await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id)).data);
            } catch {
                // Keep the current store state.
            }
            const detail = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail;
            toast.error(
                p.kind === "delete" ? "Couldn't delete the statistics history." : "Couldn't change how long statistics are kept.",
                detail ? { description: detail } : undefined,
            );
            close(p.kind === "delete" ? "menu" : "select");
        } finally {
            setBusy(false);
        }
    };

    const copy = pending ? dialogCopy(pending) : null;

    return (
        <div className="flex flex-col gap-2 items-start sm:items-end">
            <div className="flex items-center gap-2">
                <label htmlFor={`${uid}-ret`} className={cn("text-sm", mutedText)}>
                    Kept for
                </label>
                <select
                    id={`${uid}-ret`}
                    ref={selectRef}
                    value={shown}
                    disabled={isRestricted || busy}
                    aria-describedby={isRestricted ? noteId : undefined}
                    onChange={e => onSelect(e.target.value)}
                    className={cn(
                        "min-h-11 lg:min-h-9 rounded-md border border-[var(--tailwind-colors-slate-600)] bg-transparent px-3 text-sm",
                        titleText,
                        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] disabled:opacity-60 disabled:cursor-not-allowed",
                    )}
                >
                    {STATS_RETENTION_OPTIONS.map(o => (
                        <option key={o.value} value={o.value}>
                            {o.words}
                        </option>
                    ))}
                </select>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <button
                            type="button"
                            ref={menuRef}
                            aria-label="More statistics actions"
                            disabled={isRestricted || busy}
                            className={cn(
                                "inline-flex items-center justify-center min-h-11 min-w-11 lg:min-h-9 lg:min-w-9 rounded-md hover:bg-muted disabled:opacity-60 disabled:cursor-not-allowed",
                                titleText,
                                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]",
                            )}
                        >
                            <EllipsisVertical className="w-4 h-4" aria-hidden />
                        </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            variant="destructive"
                            onSelect={() => setPending({ kind: "delete" })}
                            className="min-h-11 lg:min-h-8"
                        >
                            <Trash2 className="w-4 h-4" aria-hidden />
                            Delete statistics history
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>
            {isRestricted && <LimitedAccessNote id={noteId} />}

            <Dialog open={!!pending} onOpenChange={open => !open && !busy && close(pending?.kind === "delete" ? "menu" : "select")}>
                <DialogContent
                    className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 px-4 sm:px-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]"
                    onCloseAutoFocus={e => e.preventDefault()}
                >
                    <DialogHeader className="p-6 pb-0">
                        <DialogTitle className="text-lg tracking-[-0.45px] leading-[18px] font-semibold text-[var(--tailwind-colors-slate-50)]">
                            {copy?.title}
                        </DialogTitle>
                    </DialogHeader>
                    <DialogDescription className="px-6 pt-2 pb-0 text-sm tracking-[-0.35px] leading-[19.6px] text-[var(--tailwind-colors-slate-50)]">
                        {copy?.body}
                    </DialogDescription>
                    <DialogActions>
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium"
                            disabled={busy}
                            onClick={() => close(pending?.kind === "delete" ? "menu" : "select")}
                        >
                            Cancel
                        </Button>
                        <Button
                            size="lg"
                            className={cn(
                                "flex-1 min-w-32 text-white",
                                copy?.red
                                    ? "bg-[var(--tailwind-colors-red-600)] hover:bg-[var(--tailwind-colors-red-400)]"
                                    : "bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-800)]",
                            )}
                            disabled={busy}
                            onClick={() => void confirm()}
                        >
                            {busy ? copy?.busy : copy?.confirm}
                        </Button>
                    </DialogActions>
                </DialogContent>
            </Dialog>
        </div>
    );
}
