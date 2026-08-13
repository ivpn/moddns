import { describe, test, expect } from "vitest";
import { computeNewQueryLogs, logIdentity } from "@/lib/queryLogsDiff";
import type { ModelQueryLog } from "@/api/client";

const log = (overrides: Partial<ModelQueryLog> & { domain?: string } = {}): ModelQueryLog => {
    const { domain, ...rest } = overrides;
    return {
        profile_id: "profile-1",
        timestamp: "2024-01-01T00:00:00Z",
        status: "processed",
        dns_request: { domain: domain ?? "example.com", query_type: "A" },
        device_id: "device-1",
        client_ip: "10.0.0.1",
        protocol: "udp",
        ...rest,
    } as ModelQueryLog;
};

describe("logIdentity", () => {
    test("prefers the server id when present", () => {
        expect(logIdentity(log({ id: "abc" }))).toBe("abc");
    });

    test("composite fallback discriminates same-second bursts by query type", () => {
        const a = log();
        const aaaa = { ...log(), dns_request: { domain: "example.com", query_type: "AAAA" } };
        expect(logIdentity(a)).not.toBe(logIdentity(aaaa));
    });

    test("identical entries share an identity", () => {
        expect(logIdentity(log())).toBe(logIdentity(log()));
    });
});

describe("computeNewQueryLogs", () => {
    test("returns the prefix above the first overlapping entry", () => {
        const current = [log({ domain: "c1.test" }), log({ domain: "c2.test" })];
        const fetched = [log({ domain: "n1.test" }), log({ domain: "n2.test" }), ...current];
        const diff = computeNewQueryLogs(fetched, current);
        expect(diff.newLogs.map(l => l.dns_request?.domain)).toEqual(["n1.test", "n2.test"]);
        expect(diff.overlapFound).toBe(true);
    });

    test("matches on server ids when available", () => {
        const current = [log({ id: "x1" }), log({ id: "x2" })];
        const fetched = [log({ id: "x9", domain: "new.test" }), ...current];
        const diff = computeNewQueryLogs(fetched, current);
        expect(diff.newLogs).toHaveLength(1);
        expect(diff.overlapFound).toBe(true);
    });

    test("no new entries when the head is unchanged", () => {
        const current = [log({ domain: "c1.test" }), log({ domain: "c2.test" })];
        const diff = computeNewQueryLogs([...current], current);
        expect(diff.newLogs).toHaveLength(0);
        expect(diff.overlapFound).toBe(true);
    });

    test("reports no overlap when the lists are disjoint", () => {
        const current = [log({ domain: "old.test" })];
        const fetched = [log({ domain: "n1.test" }), log({ domain: "n2.test" })];
        const diff = computeNewQueryLogs(fetched, current);
        expect(diff.newLogs).toHaveLength(2);
        expect(diff.overlapFound).toBe(false);
    });

    test("empty current list: everything is new, no overlap", () => {
        const diff = computeNewQueryLogs([log()], []);
        expect(diff.newLogs).toHaveLength(1);
        expect(diff.overlapFound).toBe(false);
    });

    test("empty fetch: nothing new, no overlap", () => {
        const diff = computeNewQueryLogs([], [log()]);
        expect(diff.newLogs).toHaveLength(0);
        expect(diff.overlapFound).toBe(false);
    });

    test("same-second burst entries only match their exact counterpart", () => {
        // A + AAAA at the same timestamp: fetching one more AAAA for a new domain must
        // not be swallowed by the timestamp-equal A entry.
        const a = log({ domain: "dup.test" });
        const current = [a];
        const aaaa = { ...log({ domain: "dup.test" }), dns_request: { domain: "dup.test", query_type: "AAAA" } };
        const diff = computeNewQueryLogs([aaaa, a], current);
        expect(diff.newLogs).toHaveLength(1);
        expect(diff.overlapFound).toBe(true);
    });
});
