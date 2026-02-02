import { useState, useEffect } from "react";
import { Select, SelectContent, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { ModelProfile } from "@/api/client/api";
import CreateProfileDialog from "@/pages/header/CreateProfileDialog";
import EditProfileDialog from "@/pages/header/EditProfileDialog";
import { DropdownMenuSeparator } from "@radix-ui/react-dropdown-menu";
import { Check, Plus, Settings } from "lucide-react";
import api from "@/api/api";
import { toast } from "sonner";

interface ProfileDropdownProps {
    profiles: ModelProfile[];
    currentProfile: ModelProfile | null;
    setActiveProfile: (profile: ModelProfile | null) => void;
    setProfiles: (profiles: ModelProfile[]) => void;
    className?: string;
}

export default function ProfileDropdown({
    profiles,
    currentProfile,
    setActiveProfile,
    setProfiles,
    className = "",
}: ProfileDropdownProps) {
    const [showCreateDialog, setShowCreateDialog] = useState(false);
    const [showEditDialog, setShowEditDialog] = useState(false);
    const [editProfile, setEditProfile] = useState<ModelProfile | null>(null);
    const [loading, setLoading] = useState(false);
    const [selectOpen, setSelectOpen] = useState(false);

    // Helper to fetch and update profiles
    const fetchProfilesAndUpdate = async (selectProfileId?: string) => {
        const response = await api.Client.profilesApi.apiV1ProfilesGet();
        setProfiles(response.data);
        if (selectProfileId) {
            const found = response.data.find((p: ModelProfile) => p.profile_id === selectProfileId);
            setActiveProfile(found || response.data[0] || null);
        }
    };

    // Handler for updating a profile (name edit)
    const handleProfileUpdated = async (updatedProfile: ModelProfile) => {
        await fetchProfilesAndUpdate(updatedProfile.profile_id);
        setShowEditDialog(false);
        setLoading(false);
    };

    // Handler for deleting a profile
    const handleProfileDeleted = async () => {
        await fetchProfilesAndUpdate();
        setShowEditDialog(false);
        setLoading(false);
        setActiveProfile(profiles[0] || null); // Set to first profile or null if none left
        currentProfile = profiles[0] || null;
    };

    // Handler for creating a profile
    const handleProfileCreated = async (newProfile: ModelProfile) => {
        await fetchProfilesAndUpdate(newProfile.profile_id);
        setShowCreateDialog(false);
        setLoading(false);
    };

    const handleProfileSwitch = (profile: ModelProfile) => {
        setActiveProfile(profile);
        setSelectOpen(false);
        toast.info(`Switched to profile: ${profile.name}`);
    };

    // Determine truncation length responsively (mobile < 768px => 16, else 20)
    const [truncateAt, setTruncateAt] = useState(16);
    useEffect(() => {
        const apply = () => {
            if (window.matchMedia('(min-width: 768px)').matches) setTruncateAt(20); else setTruncateAt(16);
        };
        apply();
        window.addEventListener('resize', apply);
        return () => window.removeEventListener('resize', apply);
    }, []);

    const truncate = (name: string): { display: string; truncated: boolean } => {
        if (name.length > truncateAt) return { display: name.slice(0, truncateAt) + '…', truncated: true };
        return { display: name, truncated: false };
    };

    return (
        <div className={`flex flex-col items-start ${className}`}>
            <Select
                value={currentProfile?.profile_id ?? ""}
                open={selectOpen}
                onOpenChange={setSelectOpen}
                onValueChange={val => {
                    const selected = profiles.find(p => p.profile_id === val);
                    if (selected) setActiveProfile(selected);
                }}
            >
                <SelectTrigger className="text-[var(--tailwind-colors-slate-50)] border-[var(--tailwind-colors-slate-600)]">
                    <SelectValue placeholder="Select profile">
                        {(() => {
                            if (!currentProfile?.name) return "Select profile";
                            const name = currentProfile.name;
                            const { display, truncated } = truncate(name);
                            return truncated ? (
                                <span title={name} data-testid="profile-name-truncated">{display}</span>
                            ) : (
                                <span data-testid="profile-name-full">{display}</span>
                            );
                        })()}
                    </SelectValue>
                </SelectTrigger>
                <SelectContent>
                    {profiles.map((profile) => {
                        const isSelected = profile.profile_id === currentProfile?.profile_id;
                        return (
                            <div
                                key={profile.profile_id}
                                className={`flex min-w-32 items-center gap-2 py-1.5 pr-2 pl-8 w-full relative cursor-pointer
                                    ${isSelected
                                        ? "bg-[var(--tailwind-colors-slate-800)]"
                                        : "bg-[var(--tailwind-colors-slate-950)]"
                                    }
                                    hover:bg-[var(--tailwind-colors-slate-800)] transition-colors duration-100
                                    rounded-[4px]`}
                                onClick={() => handleProfileSwitch(profile)}
                                data-slot="select-item"
                            >
                                {isSelected && (
                                    <Check className="absolute w-4 h-4 top-2 left-2 text-[var(--tailwind-colors-rdns-600)]" />
                                )}
                                <div className="flex-1 mt-[-1.00px] font-text-sm-leading-5-normal text-[var(--tailwind-colors-slate-50)] text-[14px] leading-[20px] overflow-hidden text-ellipsis [display:-webkit-box] [-webkit-line-clamp:1] [-webkit-box-orient:vertical]">
                                    {(() => {
                                        const { display, truncated } = truncate(profile.name);
                                        return truncated ? <span title={profile.name}>{display}</span> : profile.name;
                                    })()}
                                </div>
                                {isSelected && (
                                    <Settings
                                        className="relative w-4 h-4 text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-rdns-600)] cursor-pointer"
                                        data-testid="edit-profile-settings"
                                        onClick={e => {
                                            e.stopPropagation();
                                            setSelectOpen(false);
                                            setEditProfile(profile);
                                            setShowEditDialog(true);
                                        }}
                                    />
                                )}
                            </div>
                        );
                    })}
                    <DropdownMenuSeparator className="bg-[var(--tailwind-colors-slate-600)] h-[1px] w-full" />
                    <div
                        className="flex min-w-32 items-center gap-2 pl-2.5 pr-2 py-1.5 w-full bg-[var(--tailwind-colors-slate-950)] cursor-pointer
                            hover:bg-[var(--tailwind-colors-slate-800)] transition-colors duration-100 rounded-[4px]"
                        onClick={() => {
                            setSelectOpen(false);
                            setShowCreateDialog(true);
                        }}
                    >
                        <Plus className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)]" />
                        <div className="flex-1 mt-[-1.00px] font-text-sm-leading-5-semibold text-[var(--tailwind-colors-rdns-600)] text-[14px] leading-[20px]
                            hover:text-[var(--tailwind-colors-rdns-900)] transition-colors duration-100">
                            Create profile
                        </div>
                    </div>
                </SelectContent>
            </Select>
            <CreateProfileDialog
                open={showCreateDialog}
                onOpenChange={setShowCreateDialog}
                loading={loading}
                setLoading={setLoading}
                setActiveProfile={setActiveProfile}
                onProfileCreated={handleProfileCreated}
            />
            {showEditDialog && editProfile && (
                <EditProfileDialog
                    open={showEditDialog}
                    onOpenChange={open => {
                        setShowEditDialog(open);
                        if (!open) setEditProfile(null);
                    }}
                    profile={editProfile}
                    onProfileUpdated={handleProfileUpdated}
                    onProfileDeleted={handleProfileDeleted}
                />
            )}
        </div>
    );
}
