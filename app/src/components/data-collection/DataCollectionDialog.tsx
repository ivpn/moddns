// Opens the full control from a Statistics gate block (D4): a dialog from `md` up, a bottom sheet below.
// It never PATCHes inline; Save runs the same staged flow as everywhere else.

import type { ModelProfile } from "@/api/client";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { DataCollectionControl, type InitialFocus } from "./DataCollectionControl";
import type { DataCollectionState } from "./model";

export interface DataCollectionDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    profile: ModelProfile;
    initialPending?: Partial<DataCollectionState>;
    initialFocus?: InitialFocus;
}

export function DataCollectionDialog({ open, onOpenChange, profile, initialPending, initialFocus }: DataCollectionDialogProps) {
    const { isMobile } = useScreenDetector();
    const body = (
        <DataCollectionControl
            profile={profile}
            initialPending={initialPending}
            initialFocus={initialFocus}
            onSaved={() => onOpenChange(false)}
        />
    );
    const description = "Choose what modDNS keeps about this profile's DNS queries.";

    if (isMobile) {
        return (
            <Sheet open={open} onOpenChange={onOpenChange}>
                <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto p-4 border-[var(--tailwind-colors-slate-600)]">
                    <SheetHeader className="p-0">
                        <SheetTitle>Data collection</SheetTitle>
                        <SheetDescription>{description}</SheetDescription>
                    </SheetHeader>
                    {body}
                </SheetContent>
            </Sheet>
        );
    }
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-xl border-[var(--tailwind-colors-slate-600)]">
                <DialogHeader>
                    <DialogTitle>Data collection</DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
                {body}
            </DialogContent>
        </Dialog>
    );
}
