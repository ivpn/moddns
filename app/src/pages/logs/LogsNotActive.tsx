import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { List, Lock } from "lucide-react";
import React, { type JSX, useState } from "react";
import { useNavigate } from "react-router-dom";
import api from "@/api/api";
import { toast } from "sonner";
import type { ModelProfile } from "@/api/client";
import { useAppStore } from "@/store/general";

export const Frame = ({ profile }: { profile: ModelProfile }): JSX.Element => {
    const navigate = useNavigate();
    const [loading, setLoading] = useState(false);
    const { setActiveProfile } = useAppStore();

    const handleEnableLogs = async () => {
        setLoading(true);
        try {
            const response = await api.Client.profilesApi.apiV1ProfilesIdPatch(profile.profile_id, {
                updates: [
                    {
                        operation: "replace",
                        path: "/settings/logs/enabled",
                        value: true as unknown,
                    }
                ]
            });

            // Only update the active profile if the response is successful (HTTP 200)
            if (response.status === 200) {
                const updatedProfile = {
                    ...profile,
                    settings: {
                        ...profile.settings,
                        logs: {
                            ...profile.settings.logs,
                            enabled: true
                        }
                    }
                };
                setActiveProfile(updatedProfile);
                toast.success("Query logs enabled.");
            }
        } catch (e: unknown) {
            console.error(e);
            toast.error("Failed to enable query logs.");
        } finally {
            setLoading(false);
        }
    };

    return (
        <Card className="flex flex-col items-start relative flex-1 self-stretch w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
            <div className="flex flex-col h-[652px] items-start gap-8 p-4 pt-2 md:pt-4 relative self-stretch w-full">
                <div className="flex flex-col items-center justify-start md:justify-center gap-2.5 relative flex-1 self-stretch w-full grow -mt-4 md:mt-0">
                    <div className="flex w-12 h-12 items-center justify-center rounded-sm">
                        <div className="relative w-9 h-9 text-[var(--tailwind-colors-red-600)]">
                            <List className="absolute" strokeWidth={1.5} />
                            <Lock
                                className="absolute bottom-0 right-0 w-5 h-5"
                                strokeWidth={1.5}
                            />
                        </div>
                    </div>

                    <div className="flex flex-col items-center justify-center gap-4 relative w-full max-w-sm">
                        <h3 className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] text-center leading-7">
                            Logs are not active
                        </h3>

                        <p className="p-2 text-sm text-[var(--tailwind-colors-slate-100)] text-center font-normal font-['Roboto_Flex-Regular',Helvetica] leading-5">
                            To view DNS query logs, you need to enable logging in the settings.
                            Once enabled, your query history will appear here.
                        </p>
                    </div>

                    {/* Action button */}
                    <Button
                        className="h-auto bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] rounded-md px-6 py-2 cursor-pointer transition-colors"
                        style={{
                            '--hover-bg': 'var(--tailwind-colors-rdns-800)',
                        } as React.CSSProperties}
                        onMouseEnter={e => (e.currentTarget.style.background = 'var(--tailwind-colors-rdns-800)')}
                        onMouseLeave={e => (e.currentTarget.style.background = 'var(--tailwind-colors-rdns-600)')}
                        onClick={handleEnableLogs}
                        disabled={loading}
                    >
                        {loading ? "Enabling..." : "Enable logs"}
                    </Button>

                    <div className="p-4 flex items-center justify-center relative">
                        <div
                            className="text-xs font-medium text-[var(--shadcn-ui-app-foreground)] underline cursor-pointer"
                            onClick={() => navigate("/settings")}
                            role="button"
                            tabIndex={0}
                        >
                            Go to settings
                        </div>
                    </div>
                </div>
            </div>
        </Card>
    );
};

export default Frame;
