import { describe, beforeEach, afterEach, test, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import React from "react";
import QueryLogs from "@/pages/logs/Logs";
import { useAppStore } from "@/store/general";

// Hoisted mocks for vi.mock
const { queryLogsMock, profilesGetMock } = vi.hoisted(() => ({
    queryLogsMock: vi.fn(),
    profilesGetMock: vi.fn(),
}));

vi.mock("@/api/api", () => ({
    __esModule: true,
    default: {
        Client: {
            queryLogsApi: {
                apiV1ProfilesIdLogsGet: queryLogsMock,
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
    default: ({ log, onQuickRule, lastLogRef, isLast }: any) => {
        React.useEffect(() => {
            if (lastLogRef) {
                const el = document.createElement("div");
                lastLogRef(el as HTMLDivElement);
            }
        }, [lastLogRef, isLast]);
        return (
            <div data-testid="log-card" data-status={log.status}>
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
        onTimespanChange,
        onDeviceIdChange,
        onRefresh,
    }: any) => (
        <div>
            <input
                data-testid="search-input"
                value={searchInputValue}
                onChange={(e) => onSearchInputChange?.(e.target.value)}
            />
            <button data-testid="commit-search" onClick={() => onSearchCommit?.()}>Commit</button>
            <button data-testid="filter-blocked" onClick={() => onFilterChange?.("blocked")}>Filter Blocked</button>
            <button data-testid="timespan-all" onClick={() => onTimespanChange?.("all")}>Timespan</button>
            <button data-testid="device-select" onClick={() => onDeviceIdChange?.("device-1")}>Device</button>
            <button data-testid="refresh" onClick={() => onRefresh?.()}>Refresh</button>
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
let lastObserver: MockIntersectionObserver | null = null;

class MockIntersectionObserver {
    callback: IntersectionObserverCallback;
    constructor(callback: IntersectionObserverCallback) {
        this.callback = callback;
        lastObserver = this;
    }
    observe() { }
    unobserve() { }
    disconnect() { }
    trigger(entries: IntersectionObserverEntry[]) {
        this.callback(entries, this as unknown as IntersectionObserver);
    }
}

declare global {
    // eslint-disable-next-line no-var
    var IntersectionObserver: typeof MockIntersectionObserver;
}

global.IntersectionObserver = MockIntersectionObserver as unknown as typeof globalThis.IntersectionObserver;

const baseProfile: any = {
    profile_id: "profile-1",
    id: "profile-1",
    name: "Primary",
    settings: { logs: { enabled: true } },
};

const account: any = { id: "account-1" };

const makeLog = (overrides: Partial<any> = {}) => ({
    profile_id: baseProfile.profile_id,
    timestamp: "2024-01-01T00:00:00Z",
    status: "processed",
    dns_request: { domain: "example.com" },
    device_id: "device-123",
    protocol: "udp",
    ...overrides,
});

describe("QueryLogs", () => {
    beforeEach(() => {
        vi.useRealTimers();
        queryLogsMock.mockReset();
        profilesGetMock.mockReset();
        useAppStore.setState({ activeProfile: baseProfile });
        lastObserver = null;
    });

    afterEach(() => {
        useAppStore.setState({ activeProfile: null });
        lastObserver = null;
    });

    test("fetches with page 1 limit 100 and paginates to page 2", async () => {
        const firstPageLogs = Array.from({ length: 100 }).map((_, i) => makeLog({ timestamp: `2024-01-01T00:00:${i.toString().padStart(2, "0")}Z` }));
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: firstPageLogs });
        queryLogsMock.mockResolvedValueOnce({ status: 200, data: [] });

        render(<QueryLogs account={account} profiles={[baseProfile]} />);
        await waitFor(() => expect(queryLogsMock).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(lastObserver).toBeTruthy());
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
            lastObserver?.trigger([{ isIntersecting: true } as IntersectionObserverEntry]);
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

    test("shows not active state when logs disabled", async () => {
        const disabledProfile = { ...baseProfile, profile_id: "profile-disabled", id: "profile-disabled", settings: { logs: { enabled: false } } };
        queryLogsMock.mockResolvedValue({ status: 200, data: [] });
        useAppStore.setState({ activeProfile: disabledProfile });
        render(<QueryLogs account={account} profiles={[]} />);
        expect(await screen.findByTestId("logs-not-active")).toBeInTheDocument();
        expect(queryLogsMock).toHaveBeenCalledTimes(1);
    });
});
