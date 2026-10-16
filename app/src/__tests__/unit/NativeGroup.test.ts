import { describe, it, expect } from "vitest";
import {
    isNativeItem,
    nativeCardMatches,
    nativeCardVisible,
    nativeLogo,
    nativeSummary,
    nativeVendorMatches,
    orderedNativeItems,
} from "@/pages/blocklists/nativeGroup";
import type { ModelBlocklist } from "@/api/client/api";
import { getServiceLogoSrc } from "@/assets/services";

function bl(
    id: string,
    name: string,
    tags: string[] = ["hagezi", "native"],
    extra: Partial<ModelBlocklist> = {},
): ModelBlocklist {
    return { blocklist_id: id, name, description: `Blocks ${name} trackers`, tags, ...extra } as ModelBlocklist;
}

const samsung = bl("hagezi_native_samsung", "Samsung");
const apple = bl("hagezi_native_apple", "Apple");
const microsoft = bl("hagezi_native_winoffice", "Microsoft");
const multi = bl("hagezi_multi_pro", "Hagezi Multi PRO", ["hagezi", "comprehensive"]);
const natives = orderedNativeItems([samsung, multi, microsoft, apple]);

describe("nativeGroup membership", () => {
    it("matches only lists tagged native", () => {
        expect(isNativeItem(samsung)).toBe(true);
        expect(isNativeItem(multi)).toBe(false);
        expect(isNativeItem({ blocklist_id: "x", name: "x" } as ModelBlocklist)).toBe(false);
    });

    it("orders native lists by vendor name and drops the rest", () => {
        expect(natives.map((b) => b.name)).toEqual(["Apple", "Microsoft", "Samsung"]);
    });
});

describe("nativeGroup search", () => {
    it("treats an empty term as a card match and no vendor match", () => {
        expect(nativeCardMatches("  ")).toBe(true);
        expect(nativeVendorMatches(natives, "")).toBe(false);
    });

    it("matches the card label case-insensitively", () => {
        expect(nativeCardMatches("native TRACK")).toBe(true);
        expect(nativeCardMatches("samsung")).toBe(false);
    });

    it("matches vendor name or description", () => {
        expect(nativeVendorMatches(natives, "SAMSUNG")).toBe(true);
        expect(nativeVendorMatches(natives, "microsoft trackers")).toBe(true);
        expect(nativeVendorMatches(natives, "oisd")).toBe(false);
    });
});

describe("nativeCardVisible", () => {
    const base = { items: natives, enabled: [] as string[], searchTerm: "" };

    it.each([
        ["all", true],
        ["hagezi", true],
        ["basic", false],
        ["comprehensive", false],
        ["restrictive", false],
        ["oisd", false],
        ["native", false],
    ])("filter %s → %s", (filterValue, expected) => {
        expect(nativeCardVisible({ ...base, filterValue })).toBe(expected);
    });

    it("status enabled shows the card only when a vendor is on", () => {
        expect(nativeCardVisible({ ...base, filterValue: "enabled" })).toBe(false);
        expect(nativeCardVisible({ ...base, filterValue: "enabled", enabled: [apple.blocklist_id] })).toBe(true);
    });

    it("status disabled shows the card only when a vendor is off", () => {
        const all = natives.map((b) => b.blocklist_id);
        expect(nativeCardVisible({ ...base, filterValue: "disabled", enabled: all })).toBe(false);
        expect(nativeCardVisible({ ...base, filterValue: "disabled", enabled: all.slice(1) })).toBe(true);
    });

    it("requires a search match on the card or a vendor", () => {
        expect(nativeCardVisible({ ...base, filterValue: "all", searchTerm: "samsung" })).toBe(true);
        expect(nativeCardVisible({ ...base, filterValue: "all", searchTerm: "native" })).toBe(true);
        expect(nativeCardVisible({ ...base, filterValue: "all", searchTerm: "oisd" })).toBe(false);
        expect(nativeCardVisible({ ...base, filterValue: "basic", searchTerm: "samsung" })).toBe(false);
    });

    it("is hidden when there are no native lists", () => {
        expect(nativeCardVisible({ ...base, items: [], filterValue: "all" })).toBe(false);
    });
});

// Mirrors blocklists/sources/3rd_party/hagezi_native.json.
const SOURCE_IDS = [
    "hagezi_native_amazon",
    "hagezi_native_apple",
    "hagezi_native_huawei",
    "hagezi_native_winoffice",
    "hagezi_native_samsung",
    "hagezi_native_tiktok",
    "hagezi_native_tiktok_extended",
    "hagezi_native_lgwebos",
    "hagezi_native_roku",
    "hagezi_native_vivo",
    "hagezi_native_oppo_realme",
    "hagezi_native_xiaomi",
];

describe("nativeLogo", () => {
    it.each(SOURCE_IDS)("%s resolves to a logo in both themes", (id) => {
        const logo = nativeLogo(bl(id, id));
        expect(logo).toBeDefined();
        expect(getServiceLogoSrc({ serviceId: logo!.key, isDark: true })).toBeTruthy();
        expect(getServiceLogoSrc({ serviceId: logo!.key, isDark: false })).toBeTruthy();
    });

    it("has no logo for unknown lists", () => {
        expect(nativeLogo(multi)).toBeUndefined();
    });
});

describe("nativeSummary", () => {
    it("counts enabled lists, sums entries and picks the latest update", () => {
        const items = [
            bl("a", "A", undefined, { entries: 100, last_modified: "2026-10-01T00:00:00Z" }),
            bl("b", "B", undefined, { entries: 50, last_modified: "2026-10-09T00:00:00Z" }),
            bl("c", "C"),
        ];
        expect(nativeSummary(items, ["b", "x"])).toEqual({
            enabledCount: 1,
            totalEntries: 150,
            mostRecent: "2026-10-09T00:00:00Z",
        });
    });

    it("returns an empty mostRecent when no list has a timestamp", () => {
        expect(nativeSummary([bl("a", "A")], []).mostRecent).toBe("");
    });
});
