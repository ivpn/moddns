// "Clear query logs" (J3) for the Settings section and the cut-off Account preferences section (D5).

import { useState } from "react";
import { toast } from "sonner";
import api from "@/api/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";

/** Returns `request` to open the confirmation and the dialog element to render. */
export function useClearQueryLogs(profileId: string, { onCleared }: { onCleared?: () => void } = {}) {
    const [open, setOpen] = useState(false);
    const [busy, setBusy] = useState(false);

    const confirm = async () => {
        setBusy(true);
        try {
            await api.Client.queryLogsApi.apiV1ProfilesIdLogsDelete(profileId);
            toast.success("Query logs cleared.");
            setOpen(false);
            onCleared?.();
        } catch {
            toast.error("Failed to clear query logs.");
        } finally {
            setBusy(false);
        }
    };

    const dialog = (
        <Dialog open={open} onOpenChange={setOpen}>
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
                        onClick={() => setOpen(false)}
                        disabled={busy}
                    >
                        Cancel
                    </Button>
                    <Button
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                        onClick={() => void confirm()}
                        disabled={busy}
                    >
                        {busy ? "Clearing..." : "Clear logs"}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );

    return { request: () => setOpen(true), dialog };
}
