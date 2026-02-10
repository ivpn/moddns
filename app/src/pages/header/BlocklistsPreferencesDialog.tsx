import React, { useState, useEffect, type JSX } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DialogBody } from "@/components/dialogs/DialogLayout";
import ToggleGroup from "@/components/general/ToggleGroup";
import api from "@/api/api";
import { toast } from "sonner";
import type { ModelProfile, ModelProfileUpdateOperationEnum, ModelProfileUpdatePathEnum } from "@/api/client";

interface PreferencesDialogProps {
    currentProfile: ModelProfile;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

const initialBlocklistSettings = [
    {
        title: "Default rule",
        description: "Set the how to handle DNS queries that do not match any rules set.",
        options: [
            { value: "block", label: "Block", icon: "octagon-x" as const },
            { value: "allow", label: "Allow", icon: "check" as const },
        ],
        value: "allow",
    },
    {
        title: "Subdomains blocking",
        description: "Set how to handle subdomains of domain entries in enabled blocklists.",
        options: [
            { value: "block", label: "Block", icon: "octagon-x" as const },
            { value: "allow", label: "Allow", icon: "check" as const },
        ],
        value: "block",
    },
];

const PreferencesDialog = ({
    currentProfile,
    open,
    onOpenChange,
}: PreferencesDialogProps): JSX.Element => {
    const [blocklistSettings, setBlocklistSettings] = useState(initialBlocklistSettings);

    // Fetch current values from API when dialog opens or profile changes
    useEffect(() => {
        const fetchProfile = async () => {
            if (!currentProfile?.profile_id) return;
            try {
                const response = await api.Client.profilesApi.apiV1ProfilesIdGet(currentProfile.profile_id);
                const data = response.data;
                setBlocklistSettings([
                    {
                        ...blocklistSettings[0],
                        value: data.settings?.privacy?.default_rule ?? "allow",
                    },
                    {
                        ...blocklistSettings[1],
                        value: data.settings?.privacy?.blocklists_subdomains_rule ?? "block",
                    },
                ]);
            } catch {
                toast.error("Failed to fetch profile settings.");
            }
        };
        if (open) {
            fetchProfile();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentProfile?.profile_id, open]);

    // Generic handler for PATCH operations
    const handleProfileSettingChange = async ({
        operation = "replace",
        path,
        value,
        loadingSetter,
        stateSetter,
        idx,
        settings,
        successMessage,
        errorMessage,
    }: {
        operation?: ModelProfileUpdateOperationEnum | string;
        path: ModelProfileUpdatePathEnum | string;
        value: string | boolean;
        loadingSetter: React.Dispatch<React.SetStateAction<boolean>>;
        stateSetter: React.Dispatch<React.SetStateAction<typeof initialBlocklistSettings>>;
        idx: number;
        settings: typeof initialBlocklistSettings;
        successMessage: string;
        errorMessage: string;
    }) => {
        // Convert "enable"/"disable" to boolean for API if needed
        let apiValue: string | boolean = value;
        if (value === "enable") apiValue = true;
        else if (value === "disable") apiValue = false;
        if (value === "") return;
        if (settings[idx].value === value) return;
        loadingSetter(true);
        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(currentProfile.profile_id, {
                updates: [
                    {
                        operation: operation as ModelProfileUpdateOperationEnum,
                        path: path as ModelProfileUpdatePathEnum,
                        value: apiValue as unknown as object,
                    }
                ]
            });
            stateSetter(current =>
                current.map((setting, i) =>
                    i === idx ? { ...setting, value } : setting
                )
            );
            toast.success(successMessage || "Blocklists preferences updated successfully.");
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { detail?: string } } };
            toast.error(
                axiosErr?.response?.data?.detail ||
                errorMessage ||
                "Something went wrong while updating blocklists preferences."
            );
        } finally {
            loadingSetter(false);
        }
    };

    // Usage for blocklist
    const handleBlocklistChange = (idx: number, value: string) => {
        handleProfileSettingChange({
            path: idx === 0
                ? "/settings/privacy/default_rule"
                : "/settings/privacy/blocklists_subdomains_rule",
            value,
            loadingSetter: () => { },
            stateSetter: setBlocklistSettings,
            idx,
            settings: blocklistSettings,
            successMessage: "Blocklist setting updated.",
            errorMessage: "Failed to update blocklist setting.",
        });
    };


    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="dialog-shell w-full max-w-[calc(100vw-2rem)] sm:max-w-[680px] lg:max-w-[860px] border-[var(--tailwind-colors-slate-600)] p-0 overflow-hidden [&_[data-slot=dialog-close]_svg]:text-[var(--tailwind-colors-rdns-600)]">
                <DialogHeader className="p-6 pb-4">
                    <DialogTitle className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] tracking-[-0.45px] leading-[18px] font-['Roboto_Flex-SemiBold',Helvetica]">
                        Preferences
                    </DialogTitle>
                </DialogHeader>
                <DialogBody className="pt-0 pb-6 px-6">
                    <div className="flex flex-col items-start gap-8 w-full">
                        {blocklistSettings.map((setting, index) => (
                            <div key={index} className="flex items-center justify-between w-full gap-4">
                                <div className="flex flex-col items-start gap-2 max-w-[60%] sm:max-w-none">
                                    <h3 className="text-base font-semibold text-[var(--tailwind-colors-slate-50)] font-['Roboto_Flex-Medium',Helvetica] leading-4">
                                        {setting.title}
                                    </h3>
                                    <p className="text-sm text-[var(--tailwind-colors-slate-200)] font-text-sm-leading-5-normal leading-[var(--text-sm-leading-5-normal-line-height)]">
                                        {setting.description}
                                    </p>
                                </div>
                                <ToggleGroup
                                    options={setting.options}
                                    value={setting.value}
                                    onChange={value => handleBlocklistChange(index, value)}
                                    variant="outline"
                                    className="rounded p-0.5 shrink-0"
                                />
                            </div>
                        ))}
                    </div>
                </DialogBody>
            </DialogContent>
        </Dialog>
    );
};

export default PreferencesDialog;
