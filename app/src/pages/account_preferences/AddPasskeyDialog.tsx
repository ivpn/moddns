import { useCallback, useState, type JSX } from "react";
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
import { beginPasskeyAddReauth } from "@/lib/webauthn";
import { useAppStore } from "@/store/general";

interface AddPasskeyDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onConfirm: (verification: IdentityVerification) => void;
    loading?: boolean;
}

export default function AddPasskeyDialog({ open, onOpenChange, onConfirm, loading = false }: AddPasskeyDialogProps): JSX.Element {
    const account = useAppStore(s => s.account);
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const onVerificationChange = useCallback((v: IdentityVerification | null) => setVerification(v), []);

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="dialog-shell bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] p-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] transition-opacity duration-200 px-4 sm:px-0">
                <DialogHeader className="p-6 space-y-1.5">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                        Add passkey
                    </DialogTitle>
                    <DialogDescription className="text-sm font-normal text-[var(--tailwind-colors-slate-400)] font-['Roboto_Flex-Regular',Helvetica] leading-5">
                        Confirm it's you before adding a new passkey to your account.
                    </DialogDescription>
                </DialogHeader>

                <div className="px-6 pb-4">
                    <VerifyIdentityFields
                        account={account ?? null}
                        open={open}
                        beginPasskeyReauth={beginPasskeyAddReauth}
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
                        className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                        onClick={() => verification && onConfirm(verification)}
                        disabled={loading || !verification}
                    >
                        {loading ? "Adding..." : "Continue"}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
