import { type JSX, type LucideIcon, useMemo, useState } from "react";
import BlocklistCard from "./BlocklistCard";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import type { ModelBlocklist } from "@/api/client/api";
import { formatUpdatedRelative } from "./MainContentSection";
import {
    Dices,
    ShieldAlert,
    Heart,
    Pill,
    Users,
    Skull,
    Coins,
    AlertTriangle,
    Gamepad2,
    Globe,
    Newspaper,
    LayoutGrid,
} from "lucide-react";

const CATEGORIES: Array<{ key: string; label: string; icon: LucideIcon }> = [
    { key: "gambling", label: "Gambling", icon: Dices },
    { key: "adult", label: "Adult Content", icon: ShieldAlert },
    { key: "dating", label: "Dating", icon: Heart },
    { key: "drugs", label: "Drugs", icon: Pill },
    { key: "social_media", label: "Social Media", icon: Users },
    { key: "piracy", label: "Piracy", icon: Skull },
    { key: "crypto", label: "Cryptocurrency", icon: Coins },
    { key: "fraud", label: "Fraud & Scams", icon: AlertTriangle },
    { key: "gaming", label: "Gaming", icon: Gamepad2 },
    { key: "vpn_bypass", label: "VPN & Bypass", icon: Globe },
    { key: "clickbait", label: "Clickbait & Fake News", icon: Newspaper },
];

interface CategoriesContentSectionProps {
    blocklists: ModelBlocklist[];
    enabledBlocklists: string[];
    onToggle: (id: string, checked: boolean) => void;
    updating: string | null;
    loading: boolean;
}

export default function CategoriesContentSection({
    blocklists,
    enabledBlocklists,
    onToggle,
    updating,
    loading,
}: CategoriesContentSectionProps): JSX.Element {
    const [activeFilter, setActiveFilter] = useState<string | null>(null);

    const grouped = useMemo(() => {
        const map = new Map<string, ModelBlocklist[]>();
        for (const bl of blocklists) {
            const categoryTag = bl.tags?.[1] ?? "other";
            if (!map.has(categoryTag)) map.set(categoryTag, []);
            map.get(categoryTag)!.push(bl);
        }
        return map;
    }, [blocklists]);

    // Only show category labels that actually have blocklists
    const availableCategories = useMemo(
        () => CATEGORIES.filter((c) => (grouped.get(c.key)?.length ?? 0) > 0),
        [grouped],
    );

    const visibleGroups = useMemo(() => {
        const result: Array<{ key: string; label: string; icon: LucideIcon; items: ModelBlocklist[] }> = [];

        for (const cat of availableCategories) {
            if (activeFilter && activeFilter !== cat.key) continue;
            const items = grouped.get(cat.key);
            if (!items || items.length === 0) continue;
            result.push({ ...cat, items });
        }

        return result;
    }, [grouped, availableCategories, activeFilter]);

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Category blocklists let you block entire content categories such as gambling, adult content, or social media. Each category includes lists from multiple providers for comprehensive coverage.
                </p>
            </section>

            {/* Category labels */}
            {!loading && (
                <section className="w-full">
                    <div className="flex flex-wrap gap-2">
                        <button
                            onClick={() => setActiveFilter(null)}
                            className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors duration-150 border ${
                                activeFilter === null
                                    ? "bg-[var(--tailwind-colors-rdns-600)] text-white border-transparent"
                                    : "bg-transparent text-[var(--tailwind-colors-slate-300)] border-[var(--tailwind-colors-slate-700)] hover:border-[var(--tailwind-colors-slate-500)] hover:text-[var(--tailwind-colors-slate-100)]"
                            }`}
                        >
                            <LayoutGrid className="h-3.5 w-3.5" />
                            All
                        </button>
                        {availableCategories.map(({ key, label, icon: Icon }) => (
                            <button
                                key={key}
                                onClick={() => setActiveFilter(activeFilter === key ? null : key)}
                                className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors duration-150 border ${
                                    activeFilter === key
                                        ? "bg-[var(--tailwind-colors-rdns-600)] text-white border-transparent"
                                        : "bg-transparent text-[var(--tailwind-colors-slate-300)] border-[var(--tailwind-colors-slate-700)] hover:border-[var(--tailwind-colors-slate-500)] hover:text-[var(--tailwind-colors-slate-100)]"
                                }`}
                            >
                                <Icon className="h-3.5 w-3.5" />
                                {label}
                            </button>
                        ))}
                    </div>
                </section>
            )}

            {/* Category groups */}
            <section className="w-full">
                <ScrollArea className="w-full">
                    {loading ? (
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                            {Array.from({ length: 8 }).map((_, i) => (
                                <div key={i} className="rounded-lg border border-[var(--tailwind-colors-slate-700)] p-4 space-y-3">
                                    <div className="flex items-center justify-between">
                                        <Skeleton className="h-5 w-32" />
                                        <Skeleton className="h-5 w-10 rounded-full" />
                                    </div>
                                    <Skeleton className="h-4 w-full" />
                                    <Skeleton className="h-4 w-3/4" />
                                    <div className="flex items-center justify-between pt-2">
                                        <Skeleton className="h-3 w-20" />
                                        <Skeleton className="h-3 w-16" />
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="flex flex-col gap-8 pb-8">
                            {visibleGroups.map(({ key, label, icon: Icon, items }) => {
                                const enabledCount = items.filter((bl) =>
                                    enabledBlocklists.includes(bl.blocklist_id)
                                ).length;

                                return (
                                    <div key={key}>
                                        <div className="flex items-center gap-2 mb-4">
                                            <Icon className="h-4 w-4 text-[var(--tailwind-colors-rdns-600)]" />
                                            <h3 className="text-[var(--tailwind-colors-slate-50)] font-semibold text-base">
                                                {label}
                                            </h3>
                                            <span className="text-xs text-[var(--tailwind-colors-slate-400)]">
                                                ({enabledCount}/{items.length})
                                            </span>
                                        </div>
                                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6">
                                            {items.map((bl) => {
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
                                                        switchDisabled={updating === bl.blocklist_id}
                                                        homepage={bl.homepage}
                                                    />
                                                );
                                            })}
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </ScrollArea>
            </section>
        </div>
    );
}
