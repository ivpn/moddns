import { type JSX } from "react";
import BlocklistCard from "./BlocklistCard";
import NrdRangeCard from "./NrdRangeCard";
import { isNrdItem, orderedNrdItems } from "./nrdGroup";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import type { ModelBlocklist } from "@/api/client/api";
import { formatUpdatedRelative } from "./MainContentSection";

interface SecurityContentSectionProps {
    blocklists: ModelBlocklist[];
    enabledBlocklists: string[];
    onToggle: (id: string, checked: boolean) => void;
    /** Apply an exact target set (enable some ids, disable others) in one action. */
    onApplySet: (enableIds: string[], disableIds: string[]) => void;
    updating: string | null;
    loading: boolean;
    restricted?: boolean;
}

/**
 * Security tab — threat-protection blocklists (kind="security"). Renders the
 * Hagezi NRD windows as a single range card and every other security list (e.g.
 * Threat Intelligence Feeds) as an individual toggle card.
 */
export default function SecurityContentSection({
    blocklists,
    enabledBlocklists,
    onToggle,
    onApplySet,
    updating,
    loading,
    restricted = false,
}: SecurityContentSectionProps): JSX.Element {
    const nrdItems = orderedNrdItems(blocklists);
    const regularItems = blocklists.filter((bl) => !isNrdItem(bl));

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Security blocklists protect against malware, phishing, scams and
                    other threats. Threat Intelligence Feeds is enabled by default.
                </p>
            </section>

            <section className="w-full">
                <ScrollArea className="w-full">
                    {loading ? (
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                            {Array.from({ length: 4 }).map((_, i) => (
                                <div key={i} className="rounded-lg border border-[var(--tailwind-colors-slate-700)] p-4 space-y-3">
                                    <div className="flex items-center justify-between">
                                        <Skeleton className="h-5 w-32" />
                                        <Skeleton className="h-5 w-10 rounded-full" />
                                    </div>
                                    <Skeleton className="h-4 w-full" />
                                    <Skeleton className="h-4 w-3/4" />
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                            {nrdItems.length > 0 && (
                                <div className="col-span-1 sm:col-span-2 md:col-span-3 xl:col-span-4">
                                    <NrdRangeCard
                                        items={nrdItems}
                                        enabledBlocklists={enabledBlocklists}
                                        onApplySet={onApplySet}
                                        updating={updating}
                                        restricted={restricted}
                                    />
                                </div>
                            )}
                            {regularItems.map((bl) => {
                                const isEnabled = enabledBlocklists.includes(bl.blocklist_id);
                                return (
                                    <BlocklistCard
                                        key={bl.blocklist_id}
                                        title={bl.name}
                                        description={bl.description}
                                        entries={bl.entries}
                                        updated={formatUpdatedRelative(bl.last_modified)}
                                        onSwitchChange={(checked) => onToggle(bl.blocklist_id, checked)}
                                        switchChecked={isEnabled}
                                        switchDisabled={updating === bl.blocklist_id || restricted}
                                        homepage={bl.homepage}
                                    />
                                );
                            })}
                        </div>
                    )}
                </ScrollArea>
            </section>
        </div>
    );
}
