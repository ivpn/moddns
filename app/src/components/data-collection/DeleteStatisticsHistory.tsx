// "Delete statistics history" (S3) for the Statistics page menu and the Settings section.

import { useState } from "react";
import { toast } from "sonner";
import api from "@/api/api";
import type { ModelProfile } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import { useAppStore } from "@/store/general";

/** DELETE /profiles/{id}/statistics (204). */
async function deleteStatisticsHistory(profileId: string): Promise<void> {
    await api.Client.statisticsApi.apiV1ProfilesIdStatisticsDelete(profileId);
}

/**
 * Returns `request` to open the confirmation, the dialog element to render, and the S6 stale flag.
 * `onClosed` receives whether the history was deleted, so the caller can restore focus.
 */
export function useDeleteStatisticsHistory(
    profile: ModelProfile,
    { onDeleted, onClosed }: { onDeleted?: () => void; onClosed?: (deleted: boolean) => void } = {},
) {
    const setActiveProfile = useAppStore(s => s.setActiveProfile);
    const setProfiles = useAppStore(s => s.setProfiles);
    const [open, setOpen] = useState(false);
    const [busy, setBusy] = useState(false);
    const [stale, setStale] = useState(false);
    const savedEnabled = !!profile.settings?.statistics?.enabled;

    const writeProfile = (p: ModelProfile) => {
        setActiveProfile(p);
        setProfiles(useAppStore.getState().profiles.map(x => (x.profile_id === p.profile_id ? p : x)));
    };

    // S6: a stale tab must not confirm a delete against a state the server no longer holds.
    const request = async () => {
        setStale(false);
        setBusy(true);
        let fresh: ModelProfile | null = null;
        try {
            fresh = (await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id)).data;
        } catch {
            // A failed re-read must not block the delete.
        }
        setBusy(false);
        if (fresh && !!fresh.settings?.statistics?.enabled !== savedEnabled) {
            writeProfile(fresh);
            setStale(true);
            return;
        }
        setOpen(true);
    };

    const close = (deleted: boolean) => {
        setOpen(false);
        onClosed?.(deleted);
    };

    const confirm = async () => {
        setBusy(true);
        try {
            await deleteStatisticsHistory(profile.profile_id);
            try {
                writeProfile((await api.Client.profilesApi.apiV1ProfilesIdGet(profile.profile_id)).data);
            } catch {
                // The statistics refetch carries history_deleted_at as well.
            }
            toast.success("Statistics history deleted.");
            close(true);
            onDeleted?.();
        } catch (e) {
            const detail = (e as { response?: { data?: { detail?: string } } })?.response?.data?.detail;
            toast.error("Couldn't delete the statistics history.", detail ? { description: detail } : undefined);
            close(false);
        } finally {
            setBusy(false);
        }
    };

    const dialog = (
        <Dialog open={open} onOpenChange={o => !o && !busy && close(false)}>
            <DialogContent
                className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 px-4 sm:px-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]"
                onCloseAutoFocus={e => e.preventDefault()}
            >
                <DialogHeader className="p-6 pb-0">
                    <DialogTitle className="text-lg tracking-[-0.45px] leading-[18px] font-semibold text-[var(--tailwind-colors-slate-50)]">
                        Delete statistics history?
                    </DialogTitle>
                </DialogHeader>
                <DialogDescription className="px-6 pt-2 pb-0 text-sm tracking-[-0.35px] leading-[19.6px] text-[var(--tailwind-colors-slate-50)]">
                    All statistics for this profile will be permanently deleted. Statistics stay on and counting starts again now. This action cannot be undone.
                </DialogDescription>
                <DialogActions>
                    <Button variant="cancel" size="lg" className="flex-1 min-w-32 font-medium" disabled={busy} onClick={() => close(false)}>
                        Cancel
                    </Button>
                    <Button
                        size="lg"
                        className="flex-1 min-w-32 text-white bg-[var(--tailwind-colors-red-600)] hover:bg-[var(--tailwind-colors-red-400)]"
                        disabled={busy}
                        onClick={() => void confirm()}
                    >
                        {busy ? "Deleting…" : "Delete history"}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );

    return { request, dialog, busy, stale };
}
