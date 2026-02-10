import api from "@/api/api";
import { type JSX, useState, useEffect } from "react";
import type { ModelProfile } from "@/api/client/api";
import { ModelProfileUpdateOperationEnum, ModelProfileUpdatePathEnum } from "@/api/client/api";
import { useAppStore } from "@/store/general";
import { toast } from "sonner";
import DeleteProfileDialog from "@/pages/settings/DeleteProfileDialog";
import QueryLogsSection from "./QueryLogsSection";
import BlocklistsSection from "./BlocklistsSection";
import CustomRulesSection from "./CustomRulesSection";
import AdvancedSettingsSection from "./AdvancedSettingsSection";
import DeleteProfileSection from "./DeleteProfileSection";

interface ProfileManagementSectionProps {
    profiles: ModelProfile[];
}

export default function ProfileManagementSection({ profiles }: ProfileManagementSectionProps): JSX.Element {
    // Get active profile from store
    const activeProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);

    // Data for blocklist settings
    const [blocklistSettings, setBlocklistSettings] = useState([
        {
            title: "Default rule",
            description:
                "Set the how to handle DNS queries that do not match any rules set.",
            options: [
                { value: "block", label: "Block", icon: "octagon-x" as const },
                { value: "allow", label: "Allow", icon: "check" as const },
            ],
            value: "allow",
        },
        {
            title: "Subdomains in blocklists",
            description:
                "Set how to handle subdomains of domain entries in enabled blocklists.",
            options: [
                { value: "block", label: "Block", icon: "octagon-x" as const },
                { value: "allow", label: "Allow", icon: "check" as const },
            ],
            value: "block",
        },
    ]);

    // Data for custom rules settings
    const [customRulesSettings, setCustomRulesSettings] = useState([
        {
            title: "Subdomains in custom rules",
            description:
                "Set whether new custom rules include subdomains (*.domain) or use exact match only.",
            options: [
                { value: "exact", label: "Exact", icon: "octagon-x" as const },
                { value: "include", label: "Include", icon: "check" as const },
            ],
            value: "include",
        },
    ]);

    // Data for logs settings
    const [logsSettings, setLogsSettings] = useState([
        {
            title: "Query logs",
            description:
                "Logs are disabled by default to protect your privacy.",
            options: [
                { value: "disable", label: "Disable", icon: "octagon-x" as const },
                { value: "enable", label: "Enable", icon: "check" as const },
            ],
            value: "disable",
        },
        {
            title: "Log clients IP",
            description: "Store client IP addresses in logs.",
            options: [
                { value: "disable", label: "Disable", icon: "octagon-x" as const },
                { value: "enable", label: "Enable", icon: "check" as const },
            ],
            value: "disable",
        },
        {
            title: "Log domains",
            description: "Store queried domains in logs.",
            options: [
                { value: "disable", label: "Disable", icon: "octagon-x" as const },
                { value: "enable", label: "Enable", icon: "check" as const },
            ],
            value: "disable",
        },
        {
            title: "Retention period",
            description: "How long to keep query logs.",
            options: [
                { value: "1h", label: "1 H" },
                { value: "6h", label: "6 H" },
                { value: "1d", label: "1 D" },
                { value: "1w", label: "1 W" },
                { value: "1m", label: "1 M" },
            ],
            value: "1d",
        },
    ]);

    // Data for advanced settings
    const [advancedSettings, setAdvancedSettings] = useState([
        {
            title: "DNSSEC",
            description:
                "Validate DNSSEC-signed domains to ensure the integrity and authenticity of DNS responses.",
            options: [
                { value: "disable", label: "Disable", icon: "octagon-x" as const },
                { value: "enable", label: "Enable", icon: "check" as const },
            ],
            value: "enable",
        },
        {
            title: "DNSSEC OK (DO) bit",
            description: "Enabling the DNSSEC OK (DO) bit in DNS queries signals that the resolver supports DNSSEC.",
            options: [
                { value: "disable", label: "Disable", icon: "octagon-x" as const },
                { value: "enable", label: "Enable", icon: "check" as const },
            ],
            value: "enable",
        },
    ]);

    // Loading states for sections
    const [advancedLoading, setAdvancedLoading] = useState(false);

    // State for recursor choice
    const [currentRecursor, setCurrentRecursor] = useState("sdns");

    // Dialog state for delete profile
    const [showDeleteDialog, setShowDeleteDialog] = useState(false);

    // Usage for blocklist
    const handleBlocklistChange = async (idx: number, value: string) => {
        let apiValue: string | boolean = value;
        if (value === "enable") apiValue = true;
        else if (value === "disable") apiValue = false;
        if (value === "") return;
        if (blocklistSettings[idx].value === value) return;
        if (!activeProfile) return;

        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(activeProfile.profile_id, {
                updates: [
                    {
                        operation: ModelProfileUpdateOperationEnum.Replace,
                        path: idx === 0
                            ? ModelProfileUpdatePathEnum.SettingsPrivacyDefaultRule
                            : ModelProfileUpdatePathEnum.SettingsPrivacyBlocklistsSubdomainsRule,
                        value: apiValue as unknown as object,
                    }
                ]
            });
            setBlocklistSettings(current =>
                current.map((setting, i) =>
                    i === idx ? { ...setting, value } : setting
                )
            );
            toast.success("Blocklist setting updated.");
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { detail?: string } } };
            toast.error(axiosErr?.response?.data?.detail || "Failed to update blocklist setting.");
        }
    };

    // Usage for custom rules settings
    const handleCustomRulesSettingsChange = async (idx: number, value: string) => {
        if (value === "") return;
        if (customRulesSettings[idx].value === value) return;
        if (!activeProfile) return;

        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(activeProfile.profile_id, {
                updates: [
                    {
                        operation: ModelProfileUpdateOperationEnum.Replace,
                        path: ModelProfileUpdatePathEnum.SettingsPrivacyCustomRulesSubdomainsRule,
                        value: value as unknown as object,
                    }
                ]
            });
            setCustomRulesSettings(current =>
                current.map((setting, i) =>
                    i === idx ? { ...setting, value } : setting
                )
            );
            toast.success("Custom rules setting updated.");
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { detail?: string } } };
            toast.error(axiosErr?.response?.data?.detail || "Failed to update custom rules setting.");
        }
    };

    // Usage for logs
    const handleLogsChange = async (idx: number, value: string | boolean) => {
        let path: ModelProfileUpdatePathEnum;
        if (idx === 0) {
            path = ModelProfileUpdatePathEnum.SettingsLogsEnabled;
        } else if (idx === 1) {
            path = ModelProfileUpdatePathEnum.SettingsLogsLogClientsIps;
        } else if (idx === 2) {
            path = ModelProfileUpdatePathEnum.SettingsLogsLogDomains;
        } else if (idx === 3) {
            path = ModelProfileUpdatePathEnum.SettingsLogsRetention;
        } else {
            return; // Invalid index
        }

        let apiValue: string | boolean = value;
        if (value === "enable") apiValue = true;
        else if (value === "disable") apiValue = false;
        if (value === "") return;
        if (logsSettings[idx].value === value) return;
        if (!activeProfile) return;

        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(activeProfile.profile_id, {
                updates: [
                    {
                        operation: ModelProfileUpdateOperationEnum.Replace,
                        path,
                        value: apiValue as unknown as object,
                    }
                ]
            });
            setLogsSettings(current =>
                current.map((setting, i) =>
                    i === idx ? { ...setting, value: value as string } : setting
                )
            );
            toast.success("Logs setting updated.");
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { detail?: string } } };
            toast.error(axiosErr?.response?.data?.detail || "Failed to update logs setting.");
        }
    };

    // Usage for advanced
    const handleAdvancedChange = async (idx: number, value: string) => {
        let apiValue: string | boolean = value;
        if (value === "enable") apiValue = true;
        else if (value === "disable") apiValue = false;
        if (value === "") return;
        if (advancedSettings[idx].value === value) return;
        if (!activeProfile) return;

        setAdvancedLoading(true);
        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(activeProfile.profile_id, {
                updates: [
                    {
                        operation: ModelProfileUpdateOperationEnum.Replace,
                        path: idx === 0 ? ModelProfileUpdatePathEnum.SettingsSecurityDnssecEnabled : ModelProfileUpdatePathEnum.SettingsSecurityDnssecSendDoBit,
                        value: apiValue as unknown as object,
                    }
                ]
            });
            setAdvancedSettings(current =>
                current.map((setting, i) =>
                    i === idx ? { ...setting, value } : setting
                )
            );
            toast.success("Advanced setting updated.");
        } catch (e: unknown) {
            const axiosErr = e as { response?: { data?: { detail?: string } } };
            toast.error(axiosErr?.response?.data?.detail || "Failed to update advanced setting.");
        } finally {
            setAdvancedLoading(false);
        }
    };

    // Handler for recursor choice
    const handleRecursorChange = async (recursor: string) => {
        if (!activeProfile) return;
        if (currentRecursor === recursor) return;

        setAdvancedLoading(true);
        try {
            await api.Client.profilesApi.apiV1ProfilesIdPatch(activeProfile.profile_id, {
                updates: [
                    {
                        operation: ModelProfileUpdateOperationEnum.Replace,
                        path: ModelProfileUpdatePathEnum.SettingsAdvancedRecursor,
                        value: recursor as unknown as object,
                    }
                ]
            });
            setCurrentRecursor(recursor);
            toast.success("Recursor updated successfully.");
        } catch {
            toast.error("Failed to update recursor.");
        } finally {
            setAdvancedLoading(false);
        }
    };

    // Handler for profile deletion
    const handleProfileDeleted = () => {
        // The DeleteProfileDialog should handle the actual deletion
        // This is just a callback for when deletion is complete
        setShowDeleteDialog(false);
    };

    // Update switches state from profiles prop when page is loaded or activeProfile changes
    useEffect(() => {
        if (!activeProfile) return;

        // Find the current profile object from the profiles prop
        const profile = profiles.find(p => p.profile_id === activeProfile.profile_id);
        if (!profile) return;

        // Update blocklist settings
        setBlocklistSettings([
            {
                ...blocklistSettings[0],
                value: profile.settings?.privacy?.default_rule ?? "allow",
            },
            {
                ...blocklistSettings[1],
                value: profile.settings?.privacy?.blocklists_subdomains_rule ?? "block",
            },
        ]);

        // Update custom rules settings
        setCustomRulesSettings([
            {
                ...customRulesSettings[0],
                value: profile.settings?.privacy?.custom_rules_subdomains_rule ?? "include",
            },
        ]);

        // Update logs settings
        setLogsSettings([
            {
                ...logsSettings[0],
                value: profile.settings?.logs?.enabled ? "enable" : "disable",
            },
            {
                ...logsSettings[1],
                value: profile.settings?.logs?.log_clients_ips ? "enable" : "disable",
            },
            {
                ...logsSettings[2],
                value: profile.settings?.logs?.log_domains ? "enable" : "disable",
            },
            {
                ...logsSettings[3],
                value: profile.settings?.logs?.retention ?? "1d",
            },
        ]);

        // Update advanced settings
        setAdvancedSettings([
            {
                ...advancedSettings[0],
                value:
                    profile.settings?.security?.dnssec?.enabled === true
                        ? "enable"
                        : profile.settings?.security?.dnssec?.enabled === false
                            ? "disable"
                            : "enable",
            },
            {
                ...advancedSettings[1],
                value:
                    profile.settings?.security?.dnssec?.send_do_bit === true
                        ? "enable"
                        : profile.settings?.security?.dnssec?.send_do_bit === false
                            ? "disable"
                            : "enable",
            },
        ]);

        // Update recursor choice
        setCurrentRecursor(profile.settings?.advanced?.recursor || "sdns");
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [activeProfile, profiles]);

    return (
        <div className="flex flex-col items-start gap-4 w-full overflow-x-hidden max-w-full">
            {/* BLOCKLISTS Section */}
            <BlocklistsSection
                blocklistSettings={blocklistSettings}
                handleBlocklistChange={handleBlocklistChange}
            />

            {/* CUSTOM RULES Section */}
            <CustomRulesSection
                customRulesSettings={customRulesSettings}
                handleCustomRulesSettingsChange={handleCustomRulesSettingsChange}
            />

            {/* LOGS Section */}
            <QueryLogsSection
                logsSettings={logsSettings}
                activeProfile={activeProfile}
                handleLogsChange={handleLogsChange}
            />

            {/* ADVANCED SETTINGS Section */}
            <AdvancedSettingsSection
                advancedSettings={advancedSettings}
                advancedLoading={advancedLoading}
                handleAdvancedChange={handleAdvancedChange}
                currentRecursor={currentRecursor}
                onRecursorChange={handleRecursorChange}
            />

            {/* Delete Profile Section */}
            <DeleteProfileSection onDeleteClick={() => setShowDeleteDialog(true)} />

            {/* Delete Profile Dialog */}
            {showDeleteDialog && (
                <DeleteProfileDialog
                    open={showDeleteDialog}
                    onOpenChange={setShowDeleteDialog}
                    activeProfile={activeProfile}
                    setActiveProfile={setActiveProfile}
                    profiles={profiles}
                    onProfileDeleted={handleProfileDeleted}
                />
            )}
        </div>
    );
}