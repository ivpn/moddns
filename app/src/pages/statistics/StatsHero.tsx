import { useId } from "react";
import { Link } from "react-router-dom";
import { ChartColumn, Lock } from "lucide-react";
import type { ModelProfile } from "@/api/client";
import { DataCollectionControl } from "@/components/data-collection/DataCollectionControl";
import { StatsCard, mutedText, titleText } from "./primitives";
import { cn } from "@/lib/utils";

/** D2: the whole page when nothing is stored for the profile (P1). */
export function StatsHero({ profile, onSaved }: { profile: ModelProfile; onSaved: () => void }) {
    const headingId = useId();
    return (
        <StatsCard className="flex flex-col items-center gap-6 py-8">
            <section aria-labelledby={headingId} className="flex flex-col items-center gap-6 w-full" data-testid="stats-hero">
                <span aria-hidden className="relative w-12 h-12 text-[var(--tailwind-colors-rdns-600)]">
                    <ChartColumn className="absolute inset-1 w-10 h-10" strokeWidth={1.5} />
                    <Lock className="absolute bottom-0 right-0 w-5 h-5" strokeWidth={1.5} />
                </span>
                <div className="flex flex-col items-center gap-2 text-center">
                    <h3 id={headingId} className={cn("text-lg font-semibold leading-7", titleText)}>
                        Statistics are off
                    </h3>
                    <p className={cn("text-sm leading-5", mutedText)}>Nothing is stored for this profile.</p>
                </div>
                <div className="w-full max-w-xl">
                    <DataCollectionControl
                        profile={profile}
                        subOptions="keep"
                        labelledBy={headingId}
                        initialPending={{ level: "stats", keep: true }}
                        onSaved={onSaved}
                        keepFooter={
                            <Link
                                to="/settings"
                                className="text-sm underline text-[var(--tailwind-colors-rdns-600)] min-h-11 inline-flex items-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] rounded"
                            >
                                More options in Settings
                            </Link>
                        }
                    />
                </div>
            </section>
        </StatsCard>
    );
}
