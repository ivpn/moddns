import type { ModelBlocklist } from "@/api/client/api";

/**
 * Presentation grouping for the Hagezi Native Tracker lists.
 *
 * The backend exposes one independent `kind: general` blocklist per vendor, tagged
 * `native`. On the Lists tab they collapse into a single expandable card placed after
 * the regular lists, and stay out of the "enable listed" bulk action so each vendor
 * remains a deliberate opt-in. This module holds the grouping and visibility rules
 * as pure functions so they can be unit-tested without rendering.
 */

export const NATIVE_TAG = "native";

export const NATIVE_LABEL = "Native Trackers";
export const NATIVE_DESCRIPTION =
    "Block telemetry built into devices, apps and operating systems, per vendor, without blocking the vendor's services";

/** True when a blocklist belongs to the native tracker group. */
export function isNativeItem(bl: ModelBlocklist): boolean {
    return Array.isArray(bl.tags) && bl.tags.includes(NATIVE_TAG);
}

/** Native tracker blocklists in `items`, ordered by vendor name. */
export function orderedNativeItems(items: ModelBlocklist[]): ModelBlocklist[] {
    return items
        .filter(isNativeItem)
        .sort((a, b) => a.name.localeCompare(b.name));
}

function includesTerm(text: string | undefined, term: string): boolean {
    return (text ?? "").toLowerCase().includes(term);
}

/** True when the search term matches the group card itself (label or description). */
export function nativeCardMatches(term: string): boolean {
    const t = term.trim().toLowerCase();
    return t === "" || includesTerm(NATIVE_LABEL, t) || includesTerm(NATIVE_DESCRIPTION, t);
}

/** True when the search term matches any vendor's name or description. */
export function nativeVendorMatches(items: ModelBlocklist[], term: string): boolean {
    const t = term.trim().toLowerCase();
    if (t === "") return false;
    return items.some((bl) => includesTerm(bl.name, t) || includesTerm(bl.description, t));
}

export interface NativeVisibilityInput {
    items: ModelBlocklist[];
    enabled: Iterable<string>;
    filterValue: string;
    searchTerm: string;
}

/**
 * Whether the group card shows on the Lists tab for the current filter and search.
 * The lists carry the `hagezi` tag but no intensity, so they appear under "All lists"
 * and "Hagezi" only, never under the predefined intensity filters or other providers.
 * Status filters show the card when at least one vendor matches the status.
 */
export function nativeCardVisible({ items, enabled, filterValue, searchTerm }: NativeVisibilityInput): boolean {
    if (items.length === 0) return false;
    const enabledSet = enabled instanceof Set ? enabled : new Set(enabled);

    let matchesFilter: boolean;
    switch (filterValue) {
        case "all":
        case "hagezi":
            matchesFilter = true;
            break;
        case "enabled":
            matchesFilter = items.some((bl) => enabledSet.has(bl.blocklist_id));
            break;
        case "disabled":
            matchesFilter = items.some((bl) => !enabledSet.has(bl.blocklist_id));
            break;
        default:
            matchesFilter = false;
    }
    if (!matchesFilter) return false;

    return nativeCardMatches(searchTerm) || nativeVendorMatches(items, searchTerm);
}

export interface NativeSummary {
    enabledCount: number;
    totalEntries: number;
    /** Most recent `last_modified` across the group, or "" when none is known. */
    mostRecent: string;
}

export function nativeSummary(items: ModelBlocklist[], enabled: Iterable<string>): NativeSummary {
    const enabledSet = enabled instanceof Set ? enabled : new Set(enabled);
    let enabledCount = 0;
    let totalEntries = 0;
    let mostRecent = "";
    for (const bl of items) {
        if (enabledSet.has(bl.blocklist_id)) enabledCount++;
        if (typeof bl.entries === "number") totalEntries += bl.entries;
        if (bl.last_modified && (!mostRecent || new Date(bl.last_modified) > new Date(mostRecent))) {
            mostRecent = bl.last_modified;
        }
    }
    return { enabledCount, totalEntries, mostRecent };
}
