import React, { useState } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DialogActions } from "@/components/dialogs/DialogLayout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import api from "@/api/api";
import { toast } from "sonner";
import { useAppStore } from "@/store/general";
import { useNavigate } from "react-router-dom";
import { Label } from "@/components/ui/label";
import type { ModelProfile } from "@/api/client";

interface CreateProfileDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    loading: boolean;
    setLoading: (loading: boolean) => void;
    setActiveProfile: (profile: ModelProfile) => void;
    onProfileCreated: (profile: ModelProfile) => void;
}

export default function CreateProfileDialog({
    open,
    onOpenChange,
    loading,
    setLoading,
    setActiveProfile,
    onProfileCreated,
}: CreateProfileDialogProps) {
    const navigate = useNavigate();
    const [error, setError] = useState<string | null>(null);
    const [profileName, setProfileName] = useState("");
    const setGlobalActiveProfile = useAppStore((state) => state.setActiveProfile);

    const handleCreate = async () => {
        if (!profileName.trim()) {
            setError("Profile name cannot be empty.");
            return;
        }
        setError(null);
        setLoading(true);
        try {
            const req = { name: profileName.trim() };
            const response = await api.Client.profilesApi.apiV1ProfilesPost(req);
            if (response.status === 201 && response?.data) {
                toast.success("Profile created."); // no need to send state to Setup component since this dialog is Setup Child
                setActiveProfile(response.data);
                setGlobalActiveProfile(response.data); // Set as global active profile
                onProfileCreated(response.data);
                setProfileName("");
                onOpenChange(false);
                navigate("/setup");
            }
        } catch (err: unknown) {
            const axiosErr = err as { response?: { status?: number; data?: { error?: string } } };
            if (axiosErr?.response?.status === 400 && axiosErr?.response?.data?.error) {
                setError(axiosErr.response.data.error);
            } else {
                setError("Failed to create profile.");
            }
        } finally {
            setLoading(false);
        }
    };

    // Reset error and input when dialog closes
    const handleDialogOpenChange = (isOpen: boolean) => {
        onOpenChange(isOpen);
        if (!isOpen) {
            setError(null);
            setProfileName("");
        }
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setProfileName(e.target.value);
        // Only clear error if it was already set (prevents showing error on open)
        if (error) setError(null);
    };

    return (
        <Dialog open={open} onOpenChange={handleDialogOpenChange}>
            <DialogContent className="sm:min-w-xl w-full max-w-[calc(100vw-2rem)] border-[var(--tailwind-colors-slate-600)] p-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]">
                <DialogHeader className="p-6 pb-0">
                    <DialogTitle className="font-semibold text-lg tracking-[-0.45px] leading-[18px] text-tailwind-colors-slate-50 font-['Roboto_Flex-SemiBold',Helvetica]">
                        Create profile
                    </DialogTitle>
                </DialogHeader>
                <div className="flex flex-col gap-4 flex-1 justify-evenly h-full mt-4">
                    <div className="flex flex-col items-start gap-4 px-6 pb-4">
                        <div className="flex flex-col items-start gap-2 w-full">
                            <Label
                                htmlFor="profileName"
                                className="text-sm font-semibold text-tailwind-colors-slate-50 font-['Roboto_Flex-Medium',Helvetica]"
                            >
                                Profile name
                            </Label>
                            <Input
                                id="profileName"
                                autoFocus
                                placeholder="Type a name"
                                value={profileName}
                                onChange={handleInputChange}
                                className={`bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-sm text-[var(--tailwind-colors-slate-100)] font-['Roboto_Flex-Regular',Helvetica] h-[38px] ${error ? "border-[var(--tailwind-colors-red-600)] focus-visible:ring-[var(--tailwind-colors-red-600)]" : ""}`}
                                onKeyDown={e => {
                                    if (e.key === "Enter") handleCreate();
                                    if (e.key === "Escape") handleDialogOpenChange(false);
                                }}
                                disabled={loading}
                            />
                            {error && (
                                <span className="text-[var(--tailwind-colors-red-600)] text-xs px-1">{error}</span>
                            )}
                        </div>
                    </div>

                    <DialogActions>
                        <Button
                            variant="cancel"
                            size="lg"
                            className="flex-1 min-w-32 font-medium"
                            onClick={() => handleDialogOpenChange(false)}
                            disabled={loading}
                        >
                            Cancel
                        </Button>
                        <Button
                            variant="default"
                            size="lg"
                            className="flex-1 min-w-32 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                            onClick={handleCreate}
                            disabled={loading}
                        >
                            Create profile
                        </Button>
                    </DialogActions>
                </div>
            </DialogContent>
        </Dialog>
    );
}