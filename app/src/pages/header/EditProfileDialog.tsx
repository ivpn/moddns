import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Trash2Icon } from "lucide-react";
import React, { type JSX, useRef, useState } from "react";
import api from "@/api/api";
import { toast } from "sonner";
import DeleteProfileDialog from "@/pages/settings/DeleteProfileDialog";
import type { ModelProfile, ModelProfileUpdateOperationEnum, ModelProfileUpdatePathEnum } from "@/api/client";

export default function EditProfileDialog({
    open,
    onOpenChange,
    profile,
    onProfileUpdated,
    onProfileDeleted,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    profile: ModelProfile;
    onProfileUpdated: (updatedProfile: ModelProfile) => void;
    onProfileDeleted: (deletedProfileId: string) => void;
}): JSX.Element {
    const inputRef = useRef<HTMLInputElement>(null);
    const [editedName, setEditedName] = useState(profile.name);
    const [showSave, setShowSave] = useState(false);
    const [loading, setLoading] = useState(false);
    const [showDeleteDialog, setShowDeleteDialog] = useState(false);

    // Update editedName as user types
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setEditedName(e.target.value);
        setShowSave(
            e.target.value.trim() !== "" && e.target.value.trim() !== profile.name
        );
    };

    // Show Save button on blur if value changed
    const handleInputBlur = () => {
        setShowSave(editedName.trim() !== "" && editedName.trim() !== profile.name);
    };

    // Save handler
    const handleSave = async () => {
        if (editedName.trim() === "" || editedName.trim() === profile.name) return;
        setLoading(true);
        try {
            const req = {
                updates: [
                    {
                        operation: "replace" as ModelProfileUpdateOperationEnum,
                        path: "/name" as ModelProfileUpdatePathEnum,
                        value: editedName.trim() as unknown as object,
                    },
                ],
            };
            const response = await api.Client.profilesApi.apiV1ProfilesIdPatch(
                profile.profile_id,
                req
            );
            toast.success("Profile name updated.");
            setShowSave(false);
            onProfileUpdated(response.data); // or the updated profile object
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { error?: string } } };
            toast.error(axiosErr?.response?.data?.error || "Failed to update profile name.");
        } finally {
            setLoading(false);
        }
    };

    return (
        <>
            <Dialog open={open} onOpenChange={onOpenChange}>
                <DialogContent className="w-full max-w-3xl border-[var(--tailwind-colors-slate-600)] p-0 gap-0 [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]">
                    <DialogHeader className="p-6">
                        <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica] mt-[-1px]">
                            Edit profile
                        </DialogTitle>
                    </DialogHeader>

                    <div className="flex flex-wrap items-start gap-4 px-6 py-4 w-full">
                        <div className="flex flex-col items-start gap-8 w-full">
                            {/* Profile name section */}
                            <div className="flex items-center justify-between w-full">
                                <Label
                                    htmlFor="profileName"
                                    className="font-['Roboto_Flex-Medium',Helvetica] font-medium text-[var(--tailwind-colors-slate-50)] text-base leading-4"
                                >
                                    Profile name
                                </Label>

                                <div className="w-[227px] max-w-full flex items-center gap-2">
                                    <Input
                                        id="profileName"
                                        ref={inputRef}
                                        value={editedName}
                                        onChange={handleInputChange}
                                        onBlur={handleInputBlur}
                                        className="bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] text-sm font-['Roboto_Flex-Regular',Helvetica]"
                                    />
                                    {showSave && (
                                        <Button
                                            variant="default"
                                            size="lg"
                                            className="min-w-24 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-rdns-800)]"
                                            onClick={handleSave}
                                            disabled={loading}
                                        >
                                            {loading ? "Saving..." : "Save"}
                                        </Button>
                                    )}
                                </div>
                            </div>

                            {/* Delete profile section */}
                            <Card className="w-full bg-transparent dark:bg-[var(--danger-zone-bg)] border border-[var(--tailwind-colors-red-400)] dark:border-transparent rounded-md">
                                <CardContent className="flex items-center justify-between p-4">
                                    <div className="flex flex-col gap-2 max-w-[412px]">
                                        <h3 className="font-['Roboto_Flex-Medium',Helvetica] font-medium text-[var(--shadcn-ui-app-foreground)] text-base leading-4">
                                            Delete profile
                                        </h3>
                                        <p className="text-[var(--shadcn-ui-app-foreground)] text-sm leading-5 font-normal font-text-sm-leading-5-normal">
                                            You can delete your profile immediately,
                                            removing all associated settings and data.
                                            Account preferences and other profiles are
                                            unaffected. Cannot be reversed.
                                        </p>
                                    </div>

                                    <Button
                                        className="h-auto min-h-11 lg:min-h-0 flex items-center justify-center px-2 py-1.5 bg-[var(--tailwind-colors-red-600)] rounded-[var(--primitives-radius-radius-md)] gap-1 hover:bg-[var(--tailwind-colors-red-400)] w-full sm:w-auto min-w-40"
                                        onClick={() => setShowDeleteDialog(true)}
                                    >
                                        <Trash2Icon className="w-4 h-4 text-white" />
                                        <span className="text-white">Delete profile</span>
                                    </Button>
                                </CardContent>
                            </Card>
                        </div>
                    </div>
                </DialogContent>
            </Dialog>
            {showDeleteDialog && (
                <DeleteProfileDialog
                    open={showDeleteDialog}
                    onOpenChange={setShowDeleteDialog}
                    activeProfile={profile}
                    setActiveProfile={() => { }}
                    profiles={[]}
                    onProfileDeleted={onProfileDeleted} />
            )}
        </>
    );
}
