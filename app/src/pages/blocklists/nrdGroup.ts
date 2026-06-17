import type { ModelBlocklist } from "@/api/client/api";

/**
 * Presentation grouping for the Hagezi NRD (Newly Registered Domains) lists.
 *
 * The backend exposes the NRD windows as five independent, individually-toggleable
 * blocklists. In the UI we collapse them into a single "range" card whose stepped
 * control selects a registration-recency depth. The windows are non-overlapping
 * slices designed to be combined cumulatively, so selecting depth `k` enables the
 * first `k` tiers and disables the rest. This module holds the ordered id↔window
 * mapping and the pure depth math so it can be unit-tested without rendering.
 */

export const NRD_TAG = "nrd";

export interface NrdTier {
    id: string;
    /** Short label shown on the stepped control, e.g. "7d". */
    label: string;
    /** Cumulative window described to the user, e.g. "last 7 days". */
    window: string;
}

/** Ordered shortest→longest registration-recency window. */
export const NRD_GROUP: NrdTier[] = [
    { id: "hagezi_nrd_07", label: "7d", window: "last 7 days" },
    { id: "hagezi_nrd_14_08", label: "14d", window: "last 14 days" },
    { id: "hagezi_nrd_21_15", label: "21d", window: "last 21 days" },
    { id: "hagezi_nrd_28_22", label: "28d", window: "last 28 days" },
    { id: "hagezi_nrd_35_29", label: "35d", window: "last 35 days" },
];

const NRD_IDS = new Set(NRD_GROUP.map((t) => t.id));

/** True when a blocklist belongs to the NRD group (by tag or known id). */
export function isNrdItem(bl: ModelBlocklist): boolean {
    if (NRD_IDS.has(bl.blocklist_id)) return true;
    return Array.isArray(bl.tags) && bl.tags.includes(NRD_TAG);
}

/**
 * The NRD blocklists present in `items`, ordered shortest→longest window.
 * Tiers not present in the data are skipped (the control adapts to what exists).
 */
export function orderedNrdItems(items: ModelBlocklist[]): ModelBlocklist[] {
    const byId = new Map(items.map((bl) => [bl.blocklist_id, bl]));
    return NRD_GROUP.map((t) => byId.get(t.id)).filter(
        (bl): bl is ModelBlocklist => bl !== undefined,
    );
}

/**
 * Current selected depth = the highest enabled tier (1-based), or 0 for "Off".
 * Using the highest (rather than a contiguous count) means a gapped selection
 * still surfaces sensibly and self-heals to a contiguous set on the next change.
 */
export function nrdDepthFromEnabled(
    orderedIds: string[],
    enabled: Iterable<string>,
): number {
    const enabledSet = enabled instanceof Set ? enabled : new Set(enabled);
    let depth = 0;
    orderedIds.forEach((id, i) => {
        if (enabledSet.has(id)) depth = i + 1;
    });
    return depth;
}

/**
 * Given a target depth, the cumulative enable/disable sets over the ordered ids.
 * `enable` = first `depth` tiers; `disable` = the remainder.
 */
export function nrdDepthTargets(
    orderedIds: string[],
    depth: number,
): { enable: string[]; disable: string[] } {
    const clamped = Math.max(0, Math.min(depth, orderedIds.length));
    return {
        enable: orderedIds.slice(0, clamped),
        disable: orderedIds.slice(clamped),
    };
}
