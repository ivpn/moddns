import { type JSX, type LucideIcon, useMemo, useState, useRef, useEffect, useCallback } from "react";
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

// Display metadata for known categories (icons, labels, descriptions).
// Categories are derived from the API data — only categories present in the data are shown.
// New categories added to the backend appear automatically; add an entry here for a custom icon/label.
const CATEGORY_META: Record<string, { label: string; icon: LucideIcon; description: string }> = {
    gambling: { label: "Gambling", icon: Dices, description: "Block online casinos, betting platforms, and lottery services" },
    adult: { label: "Adult Content", icon: ShieldAlert, description: "Block adult and explicit content websites" },
    dating: { label: "Dating", icon: Heart, description: "Block dating apps and matchmaking services" },
    drugs: { label: "Drugs", icon: Pill, description: "Block sites promoting illegal drugs and substances" },
    social_media: { label: "Social Media", icon: Users, description: "Block social media platforms and networks" },
    piracy: { label: "Piracy", icon: Skull, description: "Block piracy, torrenting, and illegal streaming sites" },
    crypto: { label: "Cryptocurrency", icon: Coins, description: "Block cryptocurrency exchanges, mining, and trading platforms" },
    fraud: { label: "Fraud & Scams", icon: AlertTriangle, description: "Block phishing, scam, and fraudulent websites" },
    gaming: { label: "Gaming", icon: Gamepad2, description: "Block online gaming platforms and game-related sites" },
    vpn_bypass: { label: "VPN & Bypass", icon: Globe, description: "Block VPN services and DNS/proxy bypass tools" },
    clickbait: { label: "Clickbait & Fake News", icon: Newspaper, description: "Block clickbait, fake news, and misleading content sites" },
};

const DEFAULT_ICON = ShieldAlert;

function getCategoryMeta(key: string): { label: string; icon: LucideIcon; description: string } {
    return CATEGORY_META[key] ?? {
        label: key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
        icon: DEFAULT_ICON,
        description: `Block ${key.replace(/_/g, " ")} content`,
    };
}

function formatEntries(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
    return String(n);
}

/* ------------------------------------------------------------------ */
/*  Expand panel — full-width with two-phase animation                 */
/* ------------------------------------------------------------------ */

interface ExpandPanelProps {
    expanded: boolean;
    categoryLabel: string;
    categoryIcon: LucideIcon;
    children: React.ReactNode;
}

function ExpandPanel({ expanded, categoryLabel, categoryIcon: Icon, children }: ExpandPanelProps) {
    const innerRef = useRef<HTMLDivElement>(null);
    const [contentHeight, setContentHeight] = useState(0);
    const [spread, setSpread] = useState(false);

    useEffect(() => {
        if (expanded && innerRef.current) {
            setContentHeight(innerRef.current.scrollHeight);
        }
        if (!expanded) {
            setSpread(false);
        }
    }, [expanded, children]);

    const handleTransitionEnd = useCallback(
        (e: React.TransitionEvent) => {
            if (e.propertyName === "max-height" && expanded) {
                setSpread(true);
            }
        },
        [expanded],
    );

    return (
        <div
            className="w-full overflow-hidden transition-[max-height] duration-300 ease-out"
            style={{ maxHeight: expanded ? `${contentHeight + 48}px` : "0px" }}
            onTransitionEnd={handleTransitionEnd}
        >
            <div ref={innerRef}>
                {/* Connector bar */}
                <div className="flex items-center gap-2 pt-1 pb-3 px-1">
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[var(--tailwind-colors-slate-400)] uppercase tracking-wider select-none">
                        <Icon className="h-3 w-3 text-[var(--tailwind-colors-rdns-600)]/60" />
                        {categoryLabel}
                    </div>
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                </div>

                {/* Blocklist cards with horizontal spread animation */}
                <div
                    className={`grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4 pb-3 transition-all duration-300 ease-out ${
                        spread
                            ? "opacity-100 scale-x-100"
                            : "opacity-0 scale-x-95"
                    }`}
                    style={{ transformOrigin: "top center" }}
                >
                    {children}
                </div>

                {/* Bottom separator — closes off the expanded section */}
                <div className="flex items-center gap-2 pb-2 px-1">
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[var(--tailwind-colors-slate-400)] select-none">
                        <Icon className="h-3 w-3 text-[var(--tailwind-colors-rdns-600)]/60" />
                    </div>
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                </div>
            </div>
        </div>
    );
}

/* ------------------------------------------------------------------ */
/*  Main section                                                       */
/* ------------------------------------------------------------------ */

interface CategoriesContentSectionProps {
    blocklists: ModelBlocklist[];
    enabledBlocklists: string[];
    onToggle: (id: string, checked: boolean) => void;
    onCategoryToggle: (blocklistIds: string[], enable: boolean) => void;
    updating: string | null;
    loading: boolean;
}

interface PreparedCategory {
    key: string;
    label: string;
    icon: LucideIcon;
    description: string;
    items: ModelBlocklist[];
    recommended: ModelBlocklist[];
    enabledRecommended: number;
    totalEntries: string;
    mostRecent: string;
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

    // Group blocklists by the `category` field from the API
    const grouped = useMemo(() => {
        const map = new Map<string, ModelBlocklist[]>();
        for (const bl of blocklists) {
            const key = bl.category || "other";
            if (!map.has(key)) map.set(key, []);
            map.get(key)!.push(bl);
        }
        return map;
    }, [blocklists]);

    const getRecommended = (items: ModelBlocklist[]): ModelBlocklist[] => {
        const tagged = items.filter((bl) => bl.tags?.includes("recommended"));
        return tagged.length > 0 ? tagged : items;
    };

    // Derive categories from data — no hardcoded list needed
    const preparedCategories: PreparedCategory[] = useMemo(() => {
        return Array.from(grouped.entries())
            .filter(([key]) => key !== "other")
            .map(([key, items]) => {
                const meta = getCategoryMeta(key);
                const recommended = getRecommended(items);
                const enabledRecommended = recommended.filter((bl) =>
                    enabledBlocklists.includes(bl.blocklist_id)
                ).length;
                const totalEntries = formatEntries(
                    items.reduce((sum, bl) => sum + (typeof bl.entries === "number" ? bl.entries : 0), 0),
                );
                const mostRecent = items.reduce((latest, bl) => {
                    if (!bl.last_modified) return latest;
                    if (!latest) return bl.last_modified;
                    return new Date(bl.last_modified) > new Date(latest) ? bl.last_modified : latest;
                }, "" as string);

                return { key, ...meta, items, recommended, enabledRecommended, totalEntries, mostRecent };
            });
    }, [grouped, enabledBlocklists]);

    const handleCategoryToggle = (categoryKey: string) => {
        const items = grouped.get(categoryKey) ?? [];
        const recommended = getRecommended(items);
        const recommendedIds = recommended.map((bl) => bl.blocklist_id);
        const enabledCount = recommended.filter((bl) =>
            enabledBlocklists.includes(bl.blocklist_id)
        ).length;

        if (enabledCount >= recommended.length) {
            onCategoryToggle(recommendedIds, false);
        } else {
            onCategoryToggle(recommendedIds, true);
        }
    };

    // Find the expanded category's data for the panel
    const expandedData = expandedCategory
        ? preparedCategories.find((c) => c.key === expandedCategory)
        : null;

    // Find the index of the expanded category to know which row it's in
    const expandedIndex = expandedCategory
        ? preparedCategories.findIndex((c) => c.key === expandedCategory)
        : -1;

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Toggle content categories to quickly block entire types of content.
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
                        <CategoriesGrid
                            categories={preparedCategories}
                            expandedCategory={expandedCategory}
                            expandedIndex={expandedIndex}
                            expandedData={expandedData}
                            enabledBlocklists={enabledBlocklists}
                            updating={updating}
                            onCategoryToggle={handleCategoryToggle}
                            onExpandToggle={(key) =>
                                setExpandedCategory(expandedCategory === key ? null : key)
                            }
                            onBlocklistToggle={onToggle}
                        />
                    )}
                </ScrollArea>
            </section>
        </div>
    );
}

/* ------------------------------------------------------------------ */
/*  Grid renderer — splits categories into rows with expand panels     */
/* ------------------------------------------------------------------ */

/**
 * Uses CSS `display: contents` on a wrapper so the category cards flow
 * naturally into the parent grid, then renders the ExpandPanel as a
 * block element *outside* the grid-row wrapper so it spans full width.
 *
 * We use a useMediaQuery-like approach with fixed breakpoints to chunk
 * categories into rows. But CSS grid already handles the flow — we just
 * need to inject the expand panel after the right card.
 *
 * Simplest correct approach: render ALL category cards in the grid, then
 * use CSS `grid-column: 1 / -1` on a panel injected at the right position
 * via `order`. But CSS order can't insert *between* auto-placed items
 * reliably.
 *
 * Best approach: render the grid, then render the expand panel as a
 * separate full-width div below the grid, absolutely positioned or
 * using a portal. But that's complex.
 *
 * Cleanest approach: abandon the single grid. Render rows of cards
 * manually (flex-wrap with fixed widths) and inject expand panels
 * between rows. This gives full control.
 *
 * Actually, the cleanest approach: use `display: contents` wrappers.
 * Group [cards…, panel] where the panel has `grid-column: 1 / -1`.
 * The panel only exists in DOM when that category IS expanded, so
 * collapsed categories don't disrupt the grid.
 */

interface CategoriesGridProps {
    categories: PreparedCategory[];
    expandedCategory: string | null;
    expandedIndex: number;
    expandedData: PreparedCategory | null | undefined;
    enabledBlocklists: string[];
    updating: string | null;
    onCategoryToggle: (key: string) => void;
    onExpandToggle: (key: string) => void;
    onBlocklistToggle: (id: string, checked: boolean) => void;
}

function CategoriesGrid({
    categories,
    expandedCategory,
    expandedData,
    enabledBlocklists,
    updating,
    onCategoryToggle,
    onExpandToggle,
    onBlocklistToggle,
}: CategoriesGridProps) {
    return (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
            {categories.map((cat) => {
                const isExpanded = expandedCategory === cat.key;

                return (
                    <div key={cat.key} className="contents">
                        <CategoryCard
                            icon={cat.icon}
                            label={cat.label}
                            description={cat.description}
                            totalLists={cat.items.length}
                            enabledLists={cat.enabledRecommended}
                            totalRecommended={cat.recommended.length}
                            totalEntries={cat.totalEntries}
                            lastUpdated={formatUpdatedRelative(cat.mostRecent)}
                            onToggle={() => onCategoryToggle(cat.key)}
                            toggleDisabled={updating !== null}
                            expanded={isExpanded}
                            onExpandToggle={() => onExpandToggle(cat.key)}
                        />

                        {isExpanded && expandedData && (
                            <div className="col-[1/-1]">
                                <ExpandPanel
                                    expanded
                                    categoryLabel={expandedData.label}
                                    categoryIcon={expandedData.icon}
                                >
                                    {expandedData.items.map((bl) => {
                                        const isEnabled = enabledBlocklists.includes(bl.blocklist_id);
                                        return (
                                            <BlocklistCard
                                                key={bl.blocklist_id}
                                                title={bl.name}
                                                description={bl.description}
                                                entries={bl.entries}
                                                updated={formatUpdatedRelative(bl.last_modified)}
                                                onSwitchChange={(checked) => onBlocklistToggle(bl.blocklist_id, checked)}
                                                switchChecked={isEnabled}
                                                switchDisabled={updating === bl.blocklist_id}
                                                homepage={bl.homepage}
                                            />
                                        );
                                    })}
                                </ExpandPanel>
                            </div>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
