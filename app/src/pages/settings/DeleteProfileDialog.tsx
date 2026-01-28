import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { useState, type JSX } from "react";
import api from "@/api/api";
import { toast } from "sonner";
import { useNavigate } from "react-router-dom";
import { DialogActions } from "@/components/dialogs/DialogLayout";

interface DeleteProfileDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    activeProfile: any;
    setActiveProfile: (profile: any) => void;
    profiles: any[];
    onProfileDeleted: (profileId: string) => void;
}

export default function DeleteProfileDialog({
    open,
    onOpenChange,
    activeProfile,
    setActiveProfile,
    profiles,
    onProfileDeleted,
}: DeleteProfileDialogProps): JSX.Element {
    const [loading, setLoading] = useState(false);
    const navigate = useNavigate();

    const handleDeleteProfile = async () => {
        setLoading(true);
        try {
            const response = await api.Client.profilesApi.apiV1ProfilesIdDelete(activeProfile.profile_id);
            // Set next available profile as active, or null if none left
            if (response.status == 204) {
                const remainingProfiles = profiles.filter(p => p.profile_id !== activeProfile.profile_id);
                setActiveProfile(remainingProfiles[0] || null);
                onOpenChange(false);
                onProfileDeleted(activeProfile.profile_id);
                toast.success("Profile deleted.");
                navigate("/setup", { state: { profileDeleted: true } });
            }
        } catch (e: any) {
            toast.error(e?.response?.data?.error || "Failed to delete profile.");
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent
                className="dialog-shell border-[var(--tailwind-colors-slate-600)] p-0 transition-opacity duration-200 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)] px-4 sm:px-0"
            >
                <div className="absolute top-2.5 right-2.5">
                    <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Close"
                        /* Remove default focus ring/outline especially visible on mobile Safari/Chrome */
                        className="h-8 w-8 rounded-[var(--shadcn-ui-radius-radius-sm)] focus:outline-none focus-visible:outline-none focus:ring-0 focus-visible:ring-0 outline-none [-webkit-tap-highlight-color:transparent]"
                        onClick={() => onOpenChange(false)}
                    >
                    </Button>
                </div>

                <DialogHeader className="p-6 pb-0">
                    <DialogTitle className="text-lg tracking-[-0.45px] leading-[18px] font-semibold text-[var(--tailwind-colors-slate-50)] [font-family:'Roboto_Flex-SemiBold',Helvetica]">
                        Ready to delete this profile?
                    </DialogTitle>
                </DialogHeader>

                <DialogDescription className="px-6 pt-2 pb-0 text-sm tracking-[-0.35px] leading-[19.6px] text-[var(--tailwind-colors-slate-50)] [font-family:'Roboto_Flex-Regular',Helvetica]">
                    This action can not be undone. Your profile and all associated logs
                    and settings will be permanently deleted.
                </DialogDescription>

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
                        onClick={handleDeleteProfile}
                        disabled={loading}
                    >
                        {loading ? "Deleting..." : "Delete profile"}
                    </Button>
                </DialogActions>
            </DialogContent>
        </Dialog>
    );
}
