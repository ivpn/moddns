import { type ComponentType, type JSX, type ReactNode } from "react";
import BlocklistCard from "./BlocklistCard";
import NrdRangeCard from "./NrdRangeCard";
import { isNrdItem, orderedNrdItems } from "./nrdGroup";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { ShieldCheck, ShieldAlert } from "lucide-react";
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
    /** DNS rebinding protection per-profile toggle (settings.security.rebinding_protection.enabled). */
    rebindingEnabled: boolean;
    onRebindingToggle: (enabled: boolean) => void;
    rebindingUpdating: boolean;
}

/**
 * Group header for the Security tab. A left-aligned uppercase label with a tinted
 * icon and a trailing gradient hairline — reuses the visual vocabulary of the
 * connector bars in CategoriesContentSection so the tabs feel consistent.
 */
function SectionLabel({
    icon: Icon,
    children,
}: {
    icon: ComponentType<{ className?: string }>;
    children: ReactNode;
}): JSX.Element {
    return (
        <div className="flex items-center gap-2 px-1 w-full select-none">
            <div className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-[var(--tailwind-colors-slate-400)]">
                <Icon className="h-3.5 w-3.5 text-[var(--tailwind-colors-rdns-600)]/70" />
                {children}
            </div>
            <div className="h-px flex-1 bg-gradient-to-r from-[var(--tailwind-colors-rdns-600)]/30 via-[var(--tailwind-colors-slate-700)]/40 to-transparent" />
        </div>
    );
}

/**
 * Security tab — split into two groups: "DNS Protection" (behavioural safeguards
 * like DNS rebinding protection) and "Threat Blocklists" (subscribable domain
 * lists: the Hagezi NRD range card plus individual security lists such as Threat
 * Intelligence Feeds and CERT.pl).
 */
export default function SecurityContentSection({
    blocklists,
    enabledBlocklists,
    onToggle,
    onApplySet,
    updating,
    loading,
    restricted = false,
    rebindingEnabled,
    onRebindingToggle,
    rebindingUpdating,
}: SecurityContentSectionProps): JSX.Element {
    const nrdItems = orderedNrdItems(blocklists);
    const regularItems = blocklists.filter((bl) => !isNrdItem(bl));
    const hasBlocklists = nrdItems.length > 0 || regularItems.length > 0;

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Defend this profile against malware, phishing, scams, and
                    DNS-based attacks on your local network. Threat Intelligence
                    Feeds is enabled by default.
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
                        <div className="flex flex-col gap-4 pb-8 w-full">
                            {/* Group 1 — behavioural protections (not subscribable lists) */}
                            <SectionLabel icon={ShieldCheck}>DNS Protection</SectionLabel>
                            <Card
                                data-testid="rebinding-protection-card"
                                className="bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent rounded-[var(--tailwind-primitives-border-radius-rounded)] shadow-sm w-full overflow-hidden"
                            >
                                <CardContent className="p-0 flex items-start justify-between gap-4">
                                    <div className="flex flex-col gap-2 min-w-0">
                                        <div className="text-tailwind-colors-slate-50 font-semibold text-base leading-tight">
                                            DNS Rebinding Protection
                                        </div>
                                        <div className="font-text-xs-leading-5-normal text-[var(--tailwind-colors-slate-100)] text-xs break-words">
                                            Block responses where a public domain resolves to a private or
                                            local IP address (e.g. 192.168.x.x, 127.0.0.1), a technique used
                                            in DNS rebinding attacks to reach devices on your network.
                                        </div>
                                    </div>
                                    <Switch
                                        data-testid="rebinding-protection-switch"
                                        checked={rebindingEnabled}
                                        onCheckedChange={onRebindingToggle}
                                        disabled={rebindingUpdating || restricted}
                                        className="w-9 h-5 flex-shrink-0
                                        data-[state=unchecked]:bg-[var(--tailwind-colors-slate-700)]
                                        data-[state=checked]:bg-[var(--tailwind-colors-rdns-600)]
                                        [&>[data-slot=switch-thumb]]:bg-background
                                        data-[state=checked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-50)]
                                        data-[state=unchecked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-400)]
                                        data-[state=checked]:[&>[data-slot=switch-thumb]]:translate-x-4"
                                    />
                                </CardContent>
                            </Card>

                            {/* Group 2 — subscribable threat blocklists */}
                            {hasBlocklists && (
                                <>
                                    <SectionLabel icon={ShieldAlert}>Threat Blocklists</SectionLabel>
                                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6">
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
                                </>
                            )}
                        </div>
                    )}
                </ScrollArea>
            </section>
        </div>
    );
}
