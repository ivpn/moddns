// D5: while the account is cut off (inactive, pending_delete) only Account preferences is reachable;
// this section lets the user turn data collection off and delete what is stored (G27, J3, J52).

import { useId } from "react";
import type { ModelProfile } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useAppStore } from "@/store/general";
import { DataCollectionControl } from "./DataCollectionControl";
import { useClearQueryLogs } from "./ClearQueryLogs";
import { useDeleteStatisticsHistory } from "./DeleteStatisticsHistory";
import { CUT_OFF_TEXT, STALE_TEXT } from "./model";

const muted = "text-sm leading-5 text-[var(--tailwind-colors-slate-200)]";

function ProfileDataCollection({ profile }: { profile: ModelProfile }) {
    const headingId = useId();
    const statsOn = !!profile.settings?.statistics?.enabled;
    const logsOn = !!profile.settings?.logs?.enabled;
    const history = useDeleteStatisticsHistory(profile);
    const clear = useClearQueryLogs(profile.profile_id);
    return (
        <section aria-labelledby={headingId} className="flex flex-col gap-4 w-full border-t border-[var(--tailwind-colors-slate-600)] pt-6">
            <h3 id={headingId} className="font-bold text-base leading-5 text-[var(--tailwind-colors-slate-50)] break-words">
                {profile.name}
            </h3>
            <DataCollectionControl profile={profile} labelledBy={headingId} restrictionNote={false} />
            <div className="flex flex-col sm:flex-row sm:items-center gap-4 w-full">
                {logsOn && (
                    <Button
                        className="h-auto min-h-11 lg:min-h-0 flex items-center justify-center px-2 py-1.5 bg-[var(--tailwind-colors-red-600)] rounded-[var(--primitives-radius-radius-md)] gap-1 hover:bg-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                        onClick={clear.request}
                    >
                        <span className="text-white">Clear query logs</span>
                    </Button>
                )}
                {statsOn && (
                    <Button
                        variant="outline"
                        className="h-auto min-h-11 lg:min-h-0 px-2 py-1.5 border-[var(--tailwind-colors-red-600)] text-[var(--tailwind-colors-red-600)] dark:text-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                        disabled={history.busy}
                        onClick={() => void history.request()}
                    >
                        Delete statistics history
                    </Button>
                )}
            </div>
            {history.stale && (
                <p aria-live="polite" className={muted}>
                    {STALE_TEXT}
                </p>
            )}
            {history.dialog}
            {clear.dialog}
        </section>
    );
}

export default function CutOffDataCollection() {
    const headingId = useId();
    const profiles = useAppStore(s => s.profiles);
    const collecting = profiles.filter(p => p.settings?.statistics?.enabled || p.settings?.logs?.enabled);
    return (
        <Card
            aria-labelledby={headingId}
            role="region"
            className="w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent"
        >
            <CardContent>
                <div className="flex flex-col items-start gap-6 w-full">
                    <h2 id={headingId} className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4">
                        DATA COLLECTION
                    </h2>
                    <p className={muted}>{collecting.length > 0 ? CUT_OFF_TEXT : "Nothing is stored about your profiles' queries."}</p>
                    {collecting.map(p => (
                        <ProfileDataCollection key={p.profile_id} profile={p} />
                    ))}
                </div>
            </CardContent>
        </Card>
    );
}
