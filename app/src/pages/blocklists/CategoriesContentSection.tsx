import { type JSX, type LucideIcon, useMemo, useState } from "react";
import BlocklistCard from "./BlocklistCard";
import CategoryCard from "./CategoryCard";
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
} from "lucide-react";

const CATEGORIES: Array<{
    key: string;
    label: string;
    icon: LucideIcon;
    description: string;
}> = [
    { key: "gambling", label: "Gambling", icon: Dices, description: "Block online casinos, betting platforms, and lottery services" },
    { key: "adult", label: "Adult Content", icon: ShieldAlert, description: "Block adult and explicit content websites" },
    { key: "dating", label: "Dating", icon: Heart, description: "Block dating apps and matchmaking services" },
    { key: "drugs", label: "Drugs", icon: Pill, description: "Block sites promoting illegal drugs and substances" },
    { key: "social_media", label: "Social Media", icon: Users, description: "Block social media platforms and networks" },
    { key: "piracy", label: "Piracy", icon: Skull, description: "Block piracy, torrenting, and illegal streaming sites" },
    { key: "crypto", label: "Cryptocurrency", icon: Coins, description: "Block cryptocurrency exchanges, mining, and trading platforms" },
    { key: "fraud", label: "Fraud & Scams", icon: AlertTriangle, description: "Block phishing, scam, and fraudulent websites" },
    { key: "gaming", label: "Gaming", icon: Gamepad2, description: "Block online gaming platforms and game-related sites" },
    { key: "vpn_bypass", label: "VPN & Bypass", icon: Globe, description: "Block VPN services and DNS/proxy bypass tools" },
    { key: "clickbait", label: "Clickbait & Fake News", icon: Newspaper, description: "Block clickbait, fake news, and misleading content sites" },
];

function formatEntries(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
    return String(n);
}

interface CategoriesContentSectionProps {
    blocklists: ModelBlocklist[];
    enabledBlocklists: string[];
    onToggle: (id: string, checked: boolean) => void;
    onCategoryToggle: (blocklistIds: string[], enable: boolean) => void;
    updating: string | null;
    loading: boolean;
}

export default function CategoriesContentSection({
    blocklists,
    enabledBlocklists,
    onToggle,
    onCategoryToggle,
    updating,
    loading,
}: CategoriesContentSectionProps): JSX.Element {
    const [expandedCategory, setExpandedCategory] = useState<string | null>(null);

    const grouped = useMemo(() => {
        const map = new Map<string, ModelBlocklist[]>();
        for (const bl of blocklists) {
            const categoryTag = bl.tags?.[1] ?? "other";
            if (!map.has(categoryTag)) map.set(categoryTag, []);
            map.get(categoryTag)!.push(bl);
        }
        return map;
    }, [blocklists]);

    const availableCategories = useMemo(
        () => CATEGORIES.filter((c) => (grouped.get(c.key)?.length ?? 0) > 0),
        [grouped],
    );

    // If no blocklists in a category are tagged "recommended", treat all as recommended
    const getRecommended = (items: ModelBlocklist[]): ModelBlocklist[] => {
        const tagged = items.filter((bl) => bl.tags?.includes("recommended"));
        return tagged.length > 0 ? tagged : items;
    };

    const handleCategoryToggle = (categoryKey: string) => {
        const items = grouped.get(categoryKey) ?? [];
        const recommended = getRecommended(items);
        const recommendedIds = recommended.map((bl) => bl.blocklist_id);
        const enabledCount = recommended.filter((bl) =>
            enabledBlocklists.includes(bl.blocklist_id)
        ).length;

        if (enabledCount >= recommended.length) {
            // All recommended enabled -> disable all recommended
            onCategoryToggle(recommendedIds, false);
        } else {
            // None or partial -> enable all recommended
            onCategoryToggle(recommendedIds, true);
        }
    };

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Enable content categories to quickly block entire types of content. Each category applies recommended blocklists from multiple providers for comprehensive coverage. Expand any category to fine-tune individual lists.
                </p>
            </section>

            <section className="w-full">
                <ScrollArea className="w-full">
                    {loading ? (
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                            {Array.from({ length: 8 }).map((_, i) => (
                                <div
                                    key={i}
                                    className="bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent rounded-[var(--tailwind-primitives-border-radius-rounded)] shadow-sm flex flex-col justify-between h-[196px] lg:h-[180px] w-full"
                                >
                                    <div className="flex flex-col gap-1">
                                        <div className="flex items-start justify-between gap-2">
                                            <div className="flex items-start gap-2">
                                                <Skeleton className="h-5 w-5 mt-0.5 rounded" />
                                                <Skeleton className="h-5 w-24" />
                                            </div>
                                            <Skeleton className="h-5 w-9 rounded-full" />
                                        </div>
                                        <div className="pt-2 space-y-1.5">
                                            <Skeleton className="h-3 w-full" />
                                            <Skeleton className="h-3 w-full" />
                                            <Skeleton className="h-3 w-3/4" />
                                        </div>
                                    </div>
                                    <div className="mt-4 flex items-center justify-between">
                                        <Skeleton className="h-3 w-14" />
                                        <Skeleton className="h-3 w-20" />
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                            {availableCategories.map(({ key, label, icon, description }) => {
                                const items = grouped.get(key) ?? [];
                                const recommended = getRecommended(items);
                                const enabledRecommended = recommended.filter((bl) =>
                                    enabledBlocklists.includes(bl.blocklist_id)
                                ).length;

                                const totalEntries = items.reduce(
                                    (sum, bl) => sum + (typeof bl.entries === "number" ? bl.entries : 0),
                                    0,
                                );

                                const mostRecent = items.reduce((latest, bl) => {
                                    if (!bl.last_modified) return latest;
                                    if (!latest) return bl.last_modified;
                                    return new Date(bl.last_modified) > new Date(latest)
                                        ? bl.last_modified
                                        : latest;
                                }, "" as string);

                                const isExpanded = expandedCategory === key;

                                return (
                                    <CategoryCard
                                        key={key}
                                        icon={icon}
                                        label={label}
                                        description={description}
                                        totalLists={items.length}
                                        enabledLists={enabledRecommended}
                                        totalRecommended={recommended.length}
                                        totalEntries={formatEntries(totalEntries)}
                                        lastUpdated={formatUpdatedRelative(mostRecent)}
                                        onToggle={() => handleCategoryToggle(key)}
                                        toggleDisabled={updating !== null}
                                        expanded={isExpanded}
                                        onExpandToggle={() =>
                                            setExpandedCategory(isExpanded ? null : key)
                                        }
                                    >
                                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
                                    </CategoryCard>
                                );
                            })}
                        </div>
                    )}
                </ScrollArea>
            </section>
        </div>
    );
}
