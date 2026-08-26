import { describe, beforeEach, afterEach, test, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import React from "react";
import QueryLogs from "@/pages/logs/Logs";
import { useAppStore } from "@/store/general";

// Hoisted mocks for vi.mock
const { queryLogsMock, queryLogsDevicesMock, profilesGetMock } = vi.hoisted(() => ({
    queryLogsMock: vi.fn(),
    queryLogsDevicesMock: vi.fn(),
    profilesGetMock: vi.fn(),
}));

vi.mock("@/api/api", () => ({
    __esModule: true,
    default: {
        Client: {
            queryLogsApi: {
                apiV1ProfilesIdLogsGet: queryLogsMock,
                apiV1ProfilesIdLogsDevicesGet: queryLogsDevicesMock,
            },
            profilesApi: {
                apiV1ProfilesIdGet: profilesGetMock,
            },
        },
    },
}));

vi.mock("@/pages/logs/QuickRuleSheet", () => ({
    __esModule: true,
    default: ({ open, defaultAction }: { open: boolean; defaultAction: string }) => (
        <div data-testid="quick-rule-sheet" data-open={open} data-default-action={defaultAction} />
    ),
}));

vi.mock("@/pages/logs/QueryLogCard", () => ({
    __esModule: true,
    default: function MockQueryLogCard({ log, onQuickRule, lastLogRef, isLast, animateEntry }: { log: { status: string; dns_request?: { domain: string } }; onQuickRule?: (domain: string, action: string) => void; lastLogRef?: (el: HTMLDivElement) => void; isLast?: boolean; animateEntry?: boolean }) {
        React.useEffect(() => {
            if (lastLogRef) {
                const el = document.createElement("div");
                lastLogRef(el as HTMLDivElement);
            }
        }, [lastLogRef, isLast]);
        return (
            <div data-testid="log-card" data-status={log.status} data-animate-entry={String(Boolean(animateEntry))}>
                <button
                    aria-label="Quick custom rule"
                    onClick={() => onQuickRule?.(log.dns_request?.domain, log.status === "blocked" ? "allowlist" : "denylist")}
                    data-is-last={String(isLast)}
                />
            </div>
        );
    },
}));

vi.mock("@/pages/logs/Filters", () => ({
    __esModule: true,
    default: ({
        searchInputValue,
        onSearchInputChange,
        onSearchCommit,
        onFilterChange,
        onSortChange,
        onTimespanChange,
        onDeviceIdChange,
        onRefresh,
        onRefreshIntervalChange,
        isRefreshing,
        onSearchClear,
        onClearFilters,
        committedSearchValue,
        lastUpdatedAt,
        availableDeviceIds,
    }: { searchInputValue: string; onSearchInputChange?: (v: string) => void; onSearchCommit?: () => void; onFilterChange?: (v: string) => void; onSortChange?: (v: string) => void; onTimespanChange?: (v: string) => void; onDeviceIdChange?: (v: string) => void; onRefresh?: () => void; onRefreshIntervalChange?: (v: string) => void; isRefreshing?: boolean; onSearchClear?: () => void; onClearFilters?: () => void; committedSearchValue?: string; lastUpdatedAt?: number | null; availableDeviceIds?: string[] }) => (
        <div data-testid="filters" data-refreshing={String(Boolean(isRefreshing))} data-committed-search={committedSearchValue ?? ""} data-last-updated={String(lastUpdatedAt ?? null)} data-available-devices={(availableDeviceIds ?? []).join(",")}>
            <input
                data-testid="search-input"
                value={searchInputValue}
                onChange={(e) => onSearchInputChange?.(e.target.value)}
            />
            <button data-testid="commit-search" onClick={() => onSearchCommit?.()}>Commit</button>
            <button data-testid="filter-blocked" onClick={() => onFilterChange?.("blocked")}>Filter Blocked</button>
            <button data-testid="sort-domain" onClick={() => onSortChange?.("domain")}>Sort Domain</button>
            <button data-testid="timespan-all" onClick={() => onTimespanChange?.("all")}>Timespan</button>
            <button data-testid="device-select" onClick={() => onDeviceIdChange?.("device-1")}>Device</button>
            <button data-testid="refresh" onClick={() => onRefresh?.()}>Refresh</button>
            <button data-testid="search-clear" onClick={() => onSearchClear?.()}>Clear search</button>
            <button data-testid="clear-filters" onClick={() => onClearFilters?.()}>Clear filters</button>
            <button data-testid="auto-refresh-toggle" onClick={() => onRefreshIntervalChange?.("auto")}>Auto refresh</button>
            <button data-testid="refresh-interval-5s" onClick={() => onRefreshIntervalChange?.("5s")}>5s</button>
            <button data-testid="refresh-interval-off" onClick={() => onRefreshIntervalChange?.("off")}>Off</button>
        </div>
    ),
}));

vi.mock("@/pages/logs/NoLogs", () => ({
    __esModule: true,
    default: ({ isSearchActive }: { isSearchActive: boolean }) => (
        <div data-testid="no-logs" data-search={isSearchActive}>
            No logs
        </div>
    ),
}));

vi.mock("@/pages/logs/LogsNotActive", () => ({
    __esModule: true,
    default: () => <div data-testid="logs-not-active">Logs not active</div>,
}));

vi.mock("sonner", () => ({
    __esModule: true,
    toast: {
        error: vi.fn(),
        success: vi.fn(),
        warning: vi.fn(),
        info: vi.fn(),
    },
}));

// Minimal IntersectionObserver mock


class MockIntersectionObserver {
    callback: IntersectionObserverCallback;
    disconnected = false;
    static lastInstance: MockIntersectionObserver | null = null;
    constructor(callback: IntersectionObserverCallback) {
        this.callback = callback;
        MockIntersectionObserver.lastInstance = this;
    }
    observe() { }
    unobserve() { }
    disconnect() {
        this.disconnected = true;
    }
    trigger(entries: IntersectionObserverEntry[]) {
        // Real observers never fire after disconnect().
        if (this.disconnected) return;
        this.callback(entries, this as unknown as IntersectionObserver);
    }
}

declare global {
    // eslint-disable-next-line no-var
    var IntersectionObserver: typeof MockIntersectionObserver;
}

global.IntersectionObserver = MockIntersectionObserver as unknown as typeof globalThis.IntersectionObserver;

// jsdom's scrollTo throws "Not implemented"; the spy silences it and records calls.
const scrollToSpy = vi.spyOn(window, "scrollTo").mockImplementation(() => { });

const baseProfile = {
    profile_id: "profile-1",
    id: "profile-1",
    name: "Primary",
    settings: { logs: { enabled: true } },
} as unknown as Record<string, unknown> & { profile_id: string; settings: { logs: { enabled: boolean } } };

const account = { id: "account-1" } as unknown as Record<string, unknown>;

const makeLog = (overrides: Record<string, unknown> = {}) => ({
    profile_id: baseProfile.profile_id,
    timestamp: "2024-01-01T00:00:00Z",
    status: "processed",
    dns_request: { domain: "example.com" },
    device_id: "device-123",
    protocol: "udp",
    ...overrides,
});

// Distinct domains + timestamps so entries neither consolidate nor collide in the
// background-tick diff. `offset` keeps batches disjoint across mock responses.
const distinctLogs = (count: number, offset: number) =>
    Array.from({ length: count }).map((_, i) =>
        makeLog({
            dns_request: { domain: `d${offset + i}.example.com` },
            timestamp: `2024-01-01T00:${Math.floor((offset + i) / 60).toString().padStart(2, "0")}:${((offset + i) % 60).toString().padStart(2, "0")}Z`,
        })
    );

describe("QueryLogs", () => {
    beforeEach(() => {
        vi.useRealTimers();
        queryLogsMock.mockReset();
        queryLogsDevicesMock.mockReset();
        queryLogsDevicesMock.mockResolvedValue({ status: 200, data: [] });
        profilesGetMock.mockReset();
        useAppStore.setState({ activeProfile: baseProfile });
        MockIntersectionObserver.lastInstance = null;
        scrollToSpy.mockClear();
    });

    afterEach(() => {
        useAppStore.setState({ activeProfile: null });
        MockIntersectionObserver.lastInstance = null;
    });

    test("fetches with page 1 limit 100 and paginates to page 2", async () => {
        const firstPageLogs = Array.from({ length: 100 }).map((_, i) => makeLog({ timestamp: `2024-01-01T00:00:${i.toString().padStart(2, "0")}Z` }));
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: firstPageLogs });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: [] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(MockIntersectionObserver.lastInstance).toBeTruthy());
        expect(queryLogsMock).toHaveBeenCalledWith(
            baseProfile.profile_id,
            1,
            100,
            undefined,
            undefined,
            undefined,
            undefined,
            "created"
        );

        act(() => {
            MockIntersectionObserver.lastInstance?.trigger([{ isIntersecting: true } as IntersectionObserverEntry]);
        });

        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(2));
        expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id,
            2,
            25,
            undefined,
            undefined,
            undefined,
            undefined,
            "created"
        );
    });

    test("opens quick rule sheet with allowlist for blocked and denylist for processed", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: [makeLog({ status: "blocked" }), makeLog({ status: "processed", dns_request: { domain: "foo.test" } })] });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);

        const buttons = await screen.findAllByLabelText("Quick custom rule");
        fireEvent.click(buttons[0]);
        expect(screen.getByTestId("quick-rule-sheet")).toHaveAttribute("data-default-action", "allowlist");
        expect(screen.getByTestId("quick-rule-sheet")).toHaveAttribute("data-open", "true");

        fireEvent.click(buttons[1]);
        expect(screen.getByTestId("quick-rule-sheet")).toHaveAttribute("data-default-action", "denylist");
    });

    test("resets and refetches when filter changes", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: [makeLog()] });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));

        act(() => {
            fireEvent.click(screen.getByTestId("filter-blocked"));
        });

        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(2));
        expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id,
            1,
            100,
            "blocked",
            undefined,
            undefined,
            undefined,
            "created"
        );
    });

    test("consolidates adjacent duplicate rows into a single card under the default time sort", async () => {
        // Same domain/status/device/client_ip/protocol, differing only in query_type (A + AAAA):
        // these are sequential duplicates and collapse into one card.
        const dupA = makeLog({ dns_request: { domain: "dup.example.com", query_type: "A" }, timestamp: "2024-01-01T00:00:02Z" });
        const dupAAAA = makeLog({ dns_request: { domain: "dup.example.com", query_type: "AAAA" }, timestamp: "2024-01-01T00:00:01Z" });
        const other = makeLog({ dns_request: { domain: "other.example.com", query_type: "A" }, timestamp: "2024-01-01T00:00:00Z" });
        queryLogsMock.mockResolvedValue({ status: 200, data: [dupA, dupAAAA, other] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        // 3 raw logs → 2 cards (the A+AAAA pair merges).
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(2));
    });

    test("does not consolidate when sorted by domain", async () => {
        const dupA = makeLog({ dns_request: { domain: "dup.example.com", query_type: "A" }, timestamp: "2024-01-01T00:00:02Z" });
        const dupAAAA = makeLog({ dns_request: { domain: "dup.example.com", query_type: "AAAA" }, timestamp: "2024-01-01T00:00:01Z" });
        queryLogsMock.mockResolvedValue({ status: 200, data: [dupA, dupAAAA] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(1));

        act(() => {
            fireEvent.click(screen.getByTestId("sort-domain"));
        });

        // Under domain sort, sequential-duplicate consolidation is disabled → both rows render.
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(2));
    });

    test("keeps cards visible when a manual refresh and pagination overlap", async () => {
        // Regression test for the "invisible cards" bug: the list container is faded to
        // opacity-0 on every page-1 refresh and only restored by a 100ms setTimeout that
        // the fetch effect's cleanup cancels. An IntersectionObserver page bump inside
        // that window (opacity-0 elements still intersect) left the cards mounted and
        // clickable but permanently invisible.
        vi.useFakeTimers();
        try {
            // Call 1: initial load (page 1, limit 100 → full page, hasMore true).
            // Call 2: the manual one-shot refresh. Call 3: the observer-driven page-2 fetch.
            queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(100, 0) });
            queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(100, 200) });
            queryLogsMock.mockResolvedValueOnce({ status: 200, data: [] });

            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            // Resolve the initial fetch and let the baseline fade-in (100ms) fire.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(150);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);
            expect(screen.getAllByTestId("log-card").length).toBeGreaterThan(0);

            act(() => {
                fireEvent.click(screen.getByTestId("refresh"));
            });
            // Previous data must stay on screen while the refresh is in flight — no blank flash.
            expect(screen.getAllByTestId("log-card").length).toBeGreaterThan(0);

            // Resolve the refresh fetch (microtasks only): the fade-in restore is now pending
            // but has not fired yet — we are inside the 100ms window.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(0);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);

            // Page bump inside the window: re-runs the fetch effect, whose cleanup cancels
            // the pending fade-in on the buggy code.
            act(() => {
                MockIntersectionObserver.lastInstance?.trigger([{ isIntersecting: true } as IntersectionObserverEntry]);
            });
            // Let everything settle. Two advances: the page-2 fetch resolves during the
            // first; the fade-in timer it schedules is created in a passive effect flushed
            // at the end of that act block, so a second advance is needed for it to fire.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(500);
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(500);
            });

            // Cards exist AND nothing hides them: the invisible-but-clickable state means an
            // ancestor stuck at opacity-0 inside the scroll container.
            expect(screen.getAllByTestId("log-card").length).toBeGreaterThan(0);
            expect(screen.getByTestId("logs-scroll-container").querySelector(".opacity-0")).toBeNull();
        } finally {
            vi.useRealTimers();
        }
    });

        // tableRef: query-logs-refresh-behaviour #C1
    test("manual refresh refetches page 1 with limit 100 and replaces the list", async () => {
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(5, 0) });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(2, 100) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));

        fireEvent.click(screen.getByTestId("refresh"));

        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(2));
        expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id,
            1,
            100,
            undefined,
            undefined,
            undefined,
            undefined,
            "created"
        );
    });

        // tableRef: query-logs-refresh-behaviour #C2 #T4 #P1
    test("auto-refresh stages new entries behind a pill instead of replacing the list", async () => {
        const initial = distinctLogs(5, 0);
        const fresh = distinctLogs(2, 100);
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: initial });
        // Immediate tick fired by enabling auto-refresh: two new entries above the known head.
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: [...fresh, ...initial] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));
        // Let the initial 100ms fade-in release so the opacity assertion below can only
        // trip on a fade restarted by the tick.
        await waitFor(() =>
            expect(screen.getByTestId("logs-scroll-container").querySelector(".opacity-0")).toBeNull()
        );

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));

        const pill = await screen.findByTestId("logs-new-queries-pill");
        expect(pill).toHaveTextContent("2 new queries");
        // The tick must not have touched the list: same cards, no fade restart.
        expect(screen.getAllByTestId("log-card")).toHaveLength(5);
        expect(screen.getByTestId("logs-scroll-container").querySelector(".opacity-0")).toBeNull();

        fireEvent.click(pill);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(7));
        expect(screen.queryByTestId("logs-new-queries-pill")).toBeNull();
        // Revealing staged entries is purely client-side — no extra request, and the
        // reader's scroll position is preserved.
        expect(queryLogsMock).toHaveBeenCalledTimes(2);
        expect(scrollToSpy).not.toHaveBeenCalled();
        // Only the revealed entries play the entry animation — the prepend remounts
        // every card, so pre-existing rows must not re-animate.
        const animateFlags = screen.getAllByTestId("log-card").map(card => card.getAttribute("data-animate-entry"));
        expect(animateFlags).toEqual(["true", "true", "false", "false", "false", "false", "false"]);
    });

        // tableRef: query-logs-refresh-behaviour #P3
    test("clears staged entries when a filter changes", async () => {
        const initial = distinctLogs(5, 0);
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: initial });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: [...distinctLogs(1, 100), ...initial] });
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(3, 200) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
        await screen.findByTestId("logs-new-queries-pill");

        fireEvent.click(screen.getByTestId("filter-blocked"));
        await waitFor(() => expect(screen.queryByTestId("logs-new-queries-pill")).toBeNull());
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
    });

        // tableRef: query-logs-refresh-behaviour #C1
    test("manual refresh spins the icon for at least half a second even on instant responses", async () => {
        vi.useFakeTimers();
        try {
            queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(3, 0) });
            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(150);
            });
            expect(screen.getByTestId("filters")).toHaveAttribute("data-refreshing", "false");

            act(() => {
                fireEvent.click(screen.getByTestId("refresh"));
            });
            // The response resolves in microtasks, yet the spin must hold...
            await act(async () => {
                await vi.advanceTimersByTimeAsync(250);
            });
            expect(screen.getByTestId("filters")).toHaveAttribute("data-refreshing", "true");
            // ...until the 500ms half-rotation minimum elapses.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(350);
            });
            expect(screen.getByTestId("filters")).toHaveAttribute("data-refreshing", "false");
        } finally {
            vi.useRealTimers();
        }
    });

    // tableRef: query-logs-refresh-behaviour #C2
    test("ticks at the selected interval and stops when switched off", async () => {
        vi.useFakeTimers();
        const hiddenSpy = vi.spyOn(document, "hidden", "get").mockReturnValue(false);
        try {
            queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(3, 0) });
            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(150);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);

            // Selecting 5s fires the immediate enable tick (call 2)...
            act(() => {
                fireEvent.click(screen.getByTestId("refresh-interval-5s"));
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(0);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);

            // ...then one tick per 5s window.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(5100);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(3);

            // Off stops the loop entirely.
            act(() => {
                fireEvent.click(screen.getByTestId("refresh-interval-off"));
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(20000);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(3);
        } finally {
            hiddenSpy.mockRestore();
            vi.useRealTimers();
        }
    });

    // tableRef: query-logs-refresh-behaviour #T1
    test("skips background ticks while the tab is hidden and catches up on return", async () => {
        vi.useFakeTimers();
        const hiddenSpy = vi.spyOn(document, "hidden", "get").mockReturnValue(false);
        try {
            const initial = distinctLogs(5, 0);
            queryLogsMock.mockResolvedValue({ status: 200, data: initial });

            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(150);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);

            // Enable auto-refresh: the immediate tick is call 2.
            act(() => {
                fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(0);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);

            // Hidden tab: interval ticks self-skip without fetching.
            hiddenSpy.mockReturnValue(true);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(25000);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);

            // Back to visible: the visibilitychange handler fires an immediate catch-up tick.
            hiddenSpy.mockReturnValue(false);
            await act(async () => {
                document.dispatchEvent(new Event("visibilitychange"));
                await vi.advanceTimersByTimeAsync(0);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(3);
        } finally {
            hiddenSpy.mockRestore();
            vi.useRealTimers();
        }
    });

        // tableRef: query-logs-refresh-behaviour #T2
    test("falls back to a full replace on tick when sorted by domain", async () => {
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(5, 0) });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(4, 100) }); // sort-change refetch
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(2, 200) }); // tick fallback replace

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));

        fireEvent.click(screen.getByTestId("sort-domain"));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(4));

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
        // Non-temporal sort: the tick replaces the list wholesale; nothing is staged.
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(2));
        expect(screen.queryByTestId("logs-new-queries-pill")).toBeNull();
        expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id,
            1,
            100,
            undefined,
            undefined,
            undefined,
            undefined,
            "domain"
        );
    });

        // tableRef: query-logs-refresh-behaviour #T3
    test("applies tick data directly when the list is empty", async () => {
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: [] });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(3, 0) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await screen.findByTestId("logs-empty-state");

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
        expect(screen.queryByTestId("logs-new-queries-pill")).toBeNull();
    });

        // tableRef: query-logs-refresh-behaviour #T5 #P2
    test("shows 100+ and reloads when the tick shares nothing with the list", async () => {
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(5, 0) });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(100, 100) }); // full page, no overlap
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(100, 100) }); // reload after pill click

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
        const pill = await screen.findByTestId("logs-new-queries-pill");
        expect(pill).toHaveTextContent("100+ new queries");

        // A gapped prepend would misorder the list — the pill triggers a full reload
        // instead, landing at the top where the revealed entries are.
        fireEvent.click(pill);
        expect(scrollToSpy).toHaveBeenCalledWith(0, 0);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(3));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(100));
    });

        // tableRef: query-logs-refresh-behaviour #C1
    test("manual refresh scrolls the window back to the top", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(3, 0) });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
        expect(scrollToSpy).not.toHaveBeenCalled();

        fireEvent.click(screen.getByTestId("refresh"));
        // Instant, never smooth: a smooth scroll would sweep the pagination sentinel
        // through the viewport and chain page fetches.
        expect(scrollToSpy).toHaveBeenCalledWith(0, 0);
    });

        // tableRef: query-logs-refresh-behaviour #T2
    test("background-tick fallback under a non-created sort never scrolls the window", async () => {
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(5, 0) });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(4, 100) }); // sort-change refetch
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(2, 200) }); // tick fallback replace

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(5));

        fireEvent.click(screen.getByTestId("sort-domain"));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(4));

        fireEvent.click(screen.getByTestId("auto-refresh-toggle"));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(2));
        // The wholesale replace fires on a timer, not a user action — yanking scroll
        // every tick would make the page unreadable under domain/client_ip sort.
        expect(scrollToSpy).not.toHaveBeenCalled();
    });

        // tableRef: query-logs-refresh-behaviour #L2
    test("error-card Try again scrolls the window back to the top", async () => {
        queryLogsMock.mockRejectedValueOnce({ response: { status: 500 } });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(3, 0) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await screen.findByTestId("logs-error");

        fireEvent.click(screen.getByTestId("logs-error-retry"));
        expect(scrollToSpy).toHaveBeenCalledWith(0, 0);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
    });

    // Spec "Known accepted properties": the infinite-scroll observer is disconnected
    // while any fetch is in flight — an intersection during loading can never advance
    // the page (a stale observer used to double-increment and the superseded fetch's
    // response was silently dropped).
    test("sentinel intersection during an in-flight page fetch cannot skip a page", async () => {
        let resolvePage2: (() => void) | undefined;
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(100, 0) });
        queryLogsMock.mockImplementationOnce(
            () => new Promise(res => {
                resolvePage2 = () => res({ status: 200, data: distinctLogs(25, 100) });
            })
        );
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(10, 200) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(100));
        const sentinelObserver = MockIntersectionObserver.lastInstance;

        act(() => {
            sentinelObserver?.trigger([{ isIntersecting: true } as IntersectionObserverEntry]);
        });
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(2)); // page 2 in flight

        // A second intersection while page 2 is loading must be inert.
        act(() => {
            sentinelObserver?.trigger([{ isIntersecting: true } as IntersectionObserverEntry]);
        });

        await act(async () => {
            resolvePage2?.();
        });
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(125));
        expect(queryLogsMock).toHaveBeenCalledTimes(2);
    });

    test("search commits 500ms after typing stops, not before", async () => {
        vi.useFakeTimers();
        try {
            queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(2, 0) });
            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(600);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);

            act(() => {
                fireEvent.change(screen.getByTestId("search-input"), { target: { value: "example" } });
            });
            // Just under the debounce window: nothing committed yet.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(450);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);
            // Window elapses → one fetch with the search term.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(100);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);
            expect(queryLogsMock).toHaveBeenLastCalledWith(
                baseProfile.profile_id, 1, 100, undefined, undefined, undefined, "example", "created"
            );
        } finally {
            vi.useRealTimers();
        }
    });

    test("typing keeps postponing the debounce; Enter commits immediately", async () => {
        vi.useFakeTimers();
        try {
            queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(2, 0) });
            render(<QueryLogs account={account} profiles={[baseProfile]} />);
            await act(async () => {
                await vi.advanceTimersByTimeAsync(600);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);

            // Two keystrokes 300ms apart: the first debounce window never completes.
            act(() => {
                fireEvent.change(screen.getByTestId("search-input"), { target: { value: "exa" } });
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(300);
            });
            act(() => {
                fireEvent.change(screen.getByTestId("search-input"), { target: { value: "example" } });
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(300);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(1);

            // Enter (stub's commit button) applies without waiting.
            act(() => {
                fireEvent.click(screen.getByTestId("commit-search"));
            });
            await act(async () => {
                await vi.advanceTimersByTimeAsync(0);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);
            expect(queryLogsMock).toHaveBeenLastCalledWith(
                baseProfile.profile_id, 1, 100, undefined, undefined, undefined, "example", "created"
            );
            // The trailing debounce is a no-op after the manual commit.
            await act(async () => {
                await vi.advanceTimersByTimeAsync(600);
            });
            expect(queryLogsMock).toHaveBeenCalledTimes(2);
        } finally {
            vi.useRealTimers();
        }
    });

    test("clear search empties both pending and committed values", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(2, 0) });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));

        fireEvent.change(screen.getByTestId("search-input"), { target: { value: "example" } });
        fireEvent.click(screen.getByTestId("commit-search"));
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(2));

        fireEvent.click(screen.getByTestId("search-clear"));
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(3));
        expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id, 1, 100, undefined, undefined, undefined, undefined, "created"
        );
        expect((screen.getByTestId("search-input") as HTMLInputElement).value).toBe("");
    });

    test("clear filters resets every request parameter to defaults", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(2, 0) });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));

        fireEvent.click(screen.getByTestId("filter-blocked"));
        fireEvent.click(screen.getByTestId("device-select"));
        fireEvent.click(screen.getByTestId("sort-domain"));
        fireEvent.change(screen.getByTestId("search-input"), { target: { value: "foo" } });
        fireEvent.click(screen.getByTestId("commit-search"));
        await waitFor(() => expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id, 1, 100, "blocked", undefined, "device-1", "foo", "domain"
        ));

        fireEvent.click(screen.getByTestId("clear-filters"));
        await waitFor(() => expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id, 1, 100, undefined, undefined, undefined, undefined, "created"
        ));
        expect((screen.getByTestId("search-input") as HTMLInputElement).value).toBe("");
    });

        // tableRef: query-logs-refresh-behaviour #L1
    test("end-of-logs marker appears only when the list is exhausted", async () => {
        // Short page (5 < limit) → hasMore false → marker.
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(5, 0) });
        const { unmount } = render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getByTestId("logs-end-marker")).toBeInTheDocument());
        unmount();

        // Full page (100 = limit) → hasMore true → no marker.
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(100, 0) });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(100));
        expect(screen.queryByTestId("logs-end-marker")).toBeNull();
    });

        // tableRef: query-logs-refresh-behaviour #L2
    test("fetch failure renders the inline error card without a toast; Try again recovers", async () => {
        const { toast } = await import("sonner");
        queryLogsMock.mockRejectedValueOnce({ response: { status: 500 } });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: distinctLogs(3, 0) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        const card = await screen.findByTestId("logs-error");
        expect(card).toHaveTextContent("Server error occurred while loading logs.");
        expect(toast.error).not.toHaveBeenCalled();
        // The empty-state onboarding card must not compete with the error.
        expect(screen.queryByTestId("logs-empty-state")).toBeNull();

        fireEvent.click(screen.getByTestId("logs-error-retry"));
        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
        expect(screen.queryByTestId("logs-error")).toBeNull();
    });

        // tableRef: query-logs-refresh-behaviour #L2 #T6
    test("403 stays fully silent: no error card, no toast", async () => {
        const { toast } = await import("sonner");
        queryLogsMock.mockRejectedValue({ response: { status: 403 } });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(screen.queryByTestId("logs-error")).toBeNull());
        expect(toast.error).not.toHaveBeenCalled();
    });

        // tableRef: query-logs-refresh-behaviour #L3
    test("page description stays static; freshness flows to the filter bar after the first load", async () => {
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(3, 0) });
        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        // Before the load: no freshness yet.
        expect(screen.getByTestId("filters")).toHaveAttribute("data-last-updated", "null");
        expect(screen.getByText(/Monitor and analyze DNS queries/)).toBeInTheDocument();

        await waitFor(() => expect(screen.getAllByTestId("log-card")).toHaveLength(3));
        // The description is untouched by data loads; Filters received a timestamp.
        expect(screen.getByText(/Monitor and analyze DNS queries/)).toBeInTheDocument();
        expect(screen.getByTestId("filters").getAttribute("data-last-updated")).not.toBe("null");
    });

    // tableRef: query-logs-refresh-behaviour #D1
    test("device dropdown is the union of the server list and row-observed ids", async () => {
        queryLogsDevicesMock.mockResolvedValue({
            status: 200,
            data: [
                { device_id: "phone", last_seen: "2024-01-01T00:00:00Z" },
                { device_id: "tablet", last_seen: "2024-01-01T00:00:00Z" },
            ],
        });
        queryLogsMock.mockResolvedValue({ status: 200, data: [makeLog({ device_id: "laptop" })] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() =>
            expect(screen.getByTestId("filters").getAttribute("data-available-devices")).toBe("laptop,phone,tablet")
        );
        expect(queryLogsDevicesMock).toHaveBeenCalledWith(baseProfile.profile_id);
    });

    // tableRef: query-logs-refresh-behaviour #D4
    test("selecting a device narrows the logs but never shrinks the device list", async () => {
        queryLogsDevicesMock.mockResolvedValue({
            status: 200,
            data: [
                { device_id: "device-1", last_seen: "2024-01-01T00:00:00Z" },
                { device_id: "phone", last_seen: "2024-01-01T00:00:00Z" },
            ],
        });
        queryLogsMock.mockResolvedValue({ status: 200, data: [makeLog({ device_id: "device-1" })] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() =>
            expect(screen.getByTestId("filters").getAttribute("data-available-devices")).toBe("device-1,phone")
        );

        fireEvent.click(screen.getByTestId("device-select"));
        await waitFor(() => expect(queryLogsMock).toHaveBeenLastCalledWith(
            baseProfile.profile_id, 1, 100, undefined, undefined, "device-1", undefined, "created"
        ));
        // The regression this feature fixes: the dropdown used to collapse to the
        // selected device because the list was wiped on every filter change.
        expect(screen.getByTestId("filters").getAttribute("data-available-devices")).toBe("device-1,phone");
    });

    // tableRef: query-logs-refresh-behaviour #D3
    test("device-list fetch failure degrades silently to row-observed ids", async () => {
        queryLogsDevicesMock.mockRejectedValue(new Error("boom"));
        queryLogsMock.mockResolvedValue({ status: 200, data: [makeLog({ device_id: "laptop" })] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() =>
            expect(screen.getByTestId("filters").getAttribute("data-available-devices")).toBe("laptop")
        );
        expect(screen.queryByTestId("logs-error")).toBeNull();
    });

    // tableRef: query-logs-refresh-behaviour #D2
    test("manual refresh refetches the server device list; profile switch reloads it", async () => {
        queryLogsDevicesMock.mockResolvedValue({ status: 200, data: [] });
        queryLogsMock.mockResolvedValue({ status: 200, data: distinctLogs(2, 0) });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsDevicesMock).toHaveBeenCalledTimes(1));

        fireEvent.click(screen.getByTestId("refresh"));
        await waitFor(() => expect(queryLogsDevicesMock).toHaveBeenCalledTimes(2));

        const otherProfile = { ...baseProfile, profile_id: "profile-2", id: "profile-2" };
        act(() => {
            useAppStore.setState({ activeProfile: otherProfile });
        });
        await waitFor(() => expect(queryLogsDevicesMock).toHaveBeenLastCalledWith("profile-2"));
    });

    test("shows not active state when logs disabled", async () => {
        const disabledProfile = { ...baseProfile, profile_id: "profile-disabled", id: "profile-disabled", settings: { logs: { enabled: false } } };
        queryLogsMock.mockResolvedValue({ status: 200, data: [] });
        useAppStore.setState({ activeProfile: disabledProfile });
        render(<QueryLogs account={account} profiles={[]} />);
        expect(await screen.findByTestId("logs-not-active")).toBeInTheDocument();
        expect(queryLogsMock).toHaveBeenCalledTimes(1);
    });
});
