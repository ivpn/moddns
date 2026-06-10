import { describe, it, expect } from "vitest";
import {
    NRD_GROUP,
    nrdDepthFromEnabled,
    nrdDepthTargets,
    orderedNrdItems,
    isNrdItem,
} from "@/pages/blocklists/nrdGroup";
import type { ModelBlocklist } from "@/api/client/api";

const ids = NRD_GROUP.map((t) => t.id);

function bl(id: string, tags: string[] = []): ModelBlocklist {
    return { blocklist_id: id, tags } as unknown as ModelBlocklist;
}

describe("nrdGroup depth math", () => {
    it("reports depth 0 when nothing is enabled", () => {
        expect(nrdDepthFromEnabled(ids, [])).toBe(0);
    });

    it("reports the cumulative depth for the lowest contiguous tiers", () => {
        expect(nrdDepthFromEnabled(ids, [ids[0]])).toBe(1);
        expect(nrdDepthFromEnabled(ids, [ids[0], ids[1]])).toBe(2);
        expect(nrdDepthFromEnabled(ids, ids)).toBe(5);
    });

    it("uses the highest enabled tier when the selection is gapped", () => {
        expect(nrdDepthFromEnabled(ids, [ids[0], ids[2]])).toBe(3);
    });

    it("maps a target depth to cumulative enable/disable sets", () => {
        expect(nrdDepthTargets(ids, 0)).toEqual({ enable: [], disable: ids });
        expect(nrdDepthTargets(ids, 2)).toEqual({
            enable: [ids[0], ids[1]],
            disable: [ids[2], ids[3], ids[4]],
        });
        expect(nrdDepthTargets(ids, 5)).toEqual({ enable: ids, disable: [] });
    });

    it("clamps out-of-range depths", () => {
        expect(nrdDepthTargets(ids, 99).enable).toEqual(ids);
        expect(nrdDepthTargets(ids, -3).enable).toEqual([]);
    });
});

describe("nrdGroup item selection", () => {
    it("identifies NRD items by known id or nrd tag", () => {
        expect(isNrdItem(bl(ids[0]))).toBe(true);
        expect(isNrdItem(bl("something_else", ["nrd"]))).toBe(true);
        expect(isNrdItem(bl("hagezi_threat_intelligence_feeds_full"))).toBe(false);
    });

    it("orders present NRD items shortest→longest and skips missing tiers", () => {
        const items = [bl(ids[2]), bl(ids[0]), bl("tif")];
        expect(orderedNrdItems(items).map((b) => b.blocklist_id)).toEqual([
            ids[0],
            ids[2],
        ]);
    });
});
