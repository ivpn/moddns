import { describe, it, expect, afterEach, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { MemoryRouter } from "react-router-dom";
import Header from "@/pages/header/Header";
import ConnectionStatusHeader from "@/pages/header/ConnectionStatusHeader";
import { MobileConnectionStatusBar } from "@/pages/setup/MobileConnectionStatusBar";
import { useAppStore } from "@/store/general";

// Force desktop layout in JSDOM so the Header tests can find the
// desktop-only "DNS Status" button.
vi.mock("@/hooks/useScreenDetector", () => ({
    useScreenDetector: () => ({
        width: 1920,
        isMobile: false,
        isTablet: false,
        isDesktop: true,
        navDesktop: true,
    }),
}));

// Spy on the polled DNS-check hook to assert it is never invoked with
// `enabled: true` while in PD state. We mock only this hook (not all of axios)
// so the rest of the app's axios-based client wiring stays intact.
vi.mock("@/hooks/useDnsConnectionStatus", () => ({
    useDnsConnectionStatus: vi.fn(() => ({
        dnsCheckResponse: { status: "", profile_id: "" },
        isLoading: true,
        error: "",
        status: {
            badge: { text: "Checking...", className: "" },
            message: "Checking DNS configuration...",
            messageColor: "",
            isCurrentProfile: false,
        },
        refresh: () => { },
        enabled: false,
    })),
}));

import { useDnsConnectionStatus } from "@/hooks/useDnsConnectionStatus";
const mockedHook = vi.mocked(useDnsConnectionStatus);

afterEach(() => {
    cleanup();
    mockedHook.mockClear();
    // Reset shared store state between tests.
    useAppStore.getState().setSubscriptionStatus(null);
    useAppStore.getState().setConnectionStatusVisible(true);
});

describe("PendingDelete: DNS connection surface", () => {
    it("ConnectionStatusHeader renders nothing when subscription is pending_delete", () => {
        useAppStore.getState().setSubscriptionStatus("pending_delete");

        const { container } = render(<ConnectionStatusHeader />);

        expect(container.firstChild).toBeNull();
        // Hook may be invoked during render, but must always be called with enabled=false in PD.
        for (const call of mockedHook.mock.calls) {
            const opts = call[1] as { enabled?: boolean } | undefined;
            expect(opts?.enabled).toBe(false);
        }
    });

    it("MobileConnectionStatusBar renders nothing when subscription is pending_delete", () => {
        useAppStore.getState().setSubscriptionStatus("pending_delete");

        const { container } = render(<MobileConnectionStatusBar />);

        expect(container.firstChild).toBeNull();
        for (const call of mockedHook.mock.calls) {
            const opts = call[1] as { enabled?: boolean } | undefined;
            expect(opts?.enabled).toBe(false);
        }
    });

    it("ConnectionStatusHeader renders normally when subscription is active", () => {
        useAppStore.getState().setSubscriptionStatus("active");

        render(<ConnectionStatusHeader />);

        // Header bar root must be present, and at least one hook call must have enabled=true.
        expect(screen.getByTestId("conn-header-root")).toBeInTheDocument();
        const enabledCalls = mockedHook.mock.calls.filter((call) => {
            const opts = call[1] as { enabled?: boolean } | undefined;
            return opts?.enabled === true;
        });
        expect(enabledCalls.length).toBeGreaterThan(0);
    });

    it('Header "DNS Status" button is disabled with cursor-not-allowed when connectionStatusRestoreDisabled is true', () => {
        render(
            <MemoryRouter>
                <Header
                    profiles={[]}
                    showProfileDropdown={false}
                    showLogoutButton={true}
                    showConnectionStatusRestoreButton={true}
                    connectionStatusRestoreDisabled={true}
                    onRestoreConnectionStatus={() => { throw new Error("must not be called when disabled"); }}
                />
            </MemoryRouter>
        );

        const button = screen.getByTestId("conn-header-show");
        expect(button).toBeDisabled();
        expect(button.className).toContain("opacity-50");
        expect(button.className).toContain("pointer-events-none");
        // Wrapper span carries the cursor-not-allowed style and the explanatory tooltip.
        const wrapper = button.parentElement!;
        expect(wrapper.className).toContain("cursor-not-allowed");
        expect(wrapper.getAttribute("title")).toBe("Feature unavailable in Pending deletion mode");
    });

    it('Header "DNS Status" button is interactive when connectionStatusRestoreDisabled is false', () => {
        const onRestore = vi.fn();
        render(
            <MemoryRouter>
                <Header
                    profiles={[]}
                    showProfileDropdown={false}
                    showLogoutButton={true}
                    showConnectionStatusRestoreButton={true}
                    connectionStatusRestoreDisabled={false}
                    onRestoreConnectionStatus={onRestore}
                />
            </MemoryRouter>
        );

        const button = screen.getByTestId("conn-header-show");
        expect(button).not.toBeDisabled();
        // Wrapper span must NOT carry the cursor-not-allowed style when enabled.
        expect(button.parentElement?.className ?? "").not.toContain("cursor-not-allowed");
        expect(button.parentElement?.getAttribute("title")).toBeNull();
        button.click();
        expect(onRestore).toHaveBeenCalledTimes(1);
    });
});
