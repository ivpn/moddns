import { Card } from "@/components/ui/card";
import { List, Lock } from "lucide-react";
import { type JSX, useId } from "react";
import { useNavigate } from "react-router-dom";
import type { ModelProfile } from "@/api/client";
import { focusPageHeading } from "@/lib/focusPageHeading";
import { DataCollectionControl } from "@/components/data-collection/DataCollectionControl";

export const Frame = ({ profile }: { profile: ModelProfile }): JSX.Element => {
    const navigate = useNavigate();
    const headingId = useId();
    const statisticsOn = !!profile.settings?.statistics?.enabled;

    return (
        <Card
            data-testid="logs-not-active"
            className="flex flex-col items-start relative flex-1 self-stretch w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent"
        >
            <div className="flex flex-col md:min-h-[652px] items-center justify-center gap-6 p-4 pt-2 md:pt-4 relative self-stretch w-full">
                <div className="flex w-12 h-12 items-center justify-center rounded-sm">
                    <div className="relative w-9 h-9 text-[var(--tailwind-colors-red-600)]">
                        <List className="absolute" strokeWidth={1.5} />
                        <Lock className="absolute bottom-0 right-0 w-5 h-5" strokeWidth={1.5} />
                    </div>
                </div>

                <div className="flex flex-col items-center gap-2 w-full max-w-md">
                    <h3
                        id={headingId}
                        className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] text-center leading-7"
                    >
                        Query logs are off
                    </h3>
                    <p className="text-sm text-[var(--tailwind-colors-slate-100)] text-center font-normal leading-5">
                        {statisticsOn
                            ? "Statistics are on for this profile. Turn on query logs to see each query."
                            : "Nothing is stored for this profile. Turn on query logs to see each query here."}
                    </p>
                </div>

                <div className="w-full max-w-xl">
                    <DataCollectionControl
                        profile={profile}
                        labelledBy={headingId}
                        initialPending={{ level: "logs", keep: true }}
                        onSaved={focusPageHeading}
                    />
                </div>

                <button
                    type="button"
                    className="text-xs font-medium text-[var(--shadcn-ui-app-foreground)] underline min-h-11 px-2 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]"
                    onClick={() => navigate("/settings")}
                >
                    Go to settings
                </button>
            </div>
        </Card>
    );
};

export default Frame;
