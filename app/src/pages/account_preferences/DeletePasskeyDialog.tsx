import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import VerifyIdentityFields, { type IdentityVerification } from "@/components/dialogs/VerifyIdentityFields";
import { beginPasskeyDeleteReauth } from "@/lib/webauthn";
import { useAppStore } from "@/store/general";
import { useCallback, useState, type JSX } from "react";

interface DeletePasskeyDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onConfirm: (verification: IdentityVerification) => void;
    loading?: boolean;
    passkeyName?: string;
}

export default function DeletePasskeyDialog({
    open,
    onOpenChange,
    onConfirm,
    loading = false,
    passkeyName = "passkey",
}: DeletePasskeyDialogProps): JSX.Element {
    const account = useAppStore(s => s.account);
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const onVerificationChange = useCallback((v: IdentityVerification | null) => setVerification(v), []);
    const handleConfirm = () => {
        if (verification) onConfirm(verification);
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="dialog-shell bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] p-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] transition-opacity duration-200 px-4 sm:px-0">
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Delete passkey
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        Are you sure you want to delete "{passkeyName}"? This action cannot be undone and you will no longer be able to use this passkey to sign in.
                    </DialogDescription>
                </DialogHeader>

                <div className="px-6 pb-4">
                    <VerifyIdentityFields
                        account={account ?? null}
                        open={open}
                        beginPasskeyReauth={beginPasskeyDeleteReauth}
                        onChange={onVerificationChange}
                        disabled={loading}
                    />
                </div>

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
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-red-600)] text-white hover:bg-[var(--tailwind-colors-red-400)]"
                        onClick={handleConfirm}
                        disabled={loading || !verification}
                    >
                        {loading ? "Deleting..." : "Delete passkey"}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
