import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import { type JSX } from "react";
import api from "@/api/api";
import { toast } from "sonner";

interface DeleteSessionsProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    loading?: boolean;
}

export default function DeleteSessionsDialog({
    open,
    onOpenChange,
    loading = false,
}: DeleteSessionsProps): JSX.Element {
    const handleConfirm = async () => {
        try {
            await api.Client.sessionsApi.apiV1SessionsDelete();
            toast.success("All other sessions have been logged out.");
            onOpenChange(false);
        } catch {
            toast.error("Failed to log out sessions.");
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] px-4 sm:px-0">
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Log out other web sessions?
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        This will log out all other web sessions except your current one. You will remain logged in on this device.
                    </DialogDescription>
                </DialogHeader>
                <DialogActions>
                    <Button
                        variant="cancel"
                        size="lg"
                        className="flex-1 min-w-32 font-medium"
                        onClick={() => onOpenChange(false)}
                        disabled={loading}
                    >
                        Cancel
                    </Button>
                    <Button
                        variant="default"
                        size="lg"
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                        onClick={handleConfirm}
                        disabled={loading}
                    >
                        Log out sessions
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
