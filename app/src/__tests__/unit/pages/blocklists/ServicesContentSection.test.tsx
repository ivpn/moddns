import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import React from "react";
import ServicesContentSection from "@/pages/blocklists/ServicesContentSection";
import { useAppStore } from "@/store/general";
import type { ModelProfile } from "@/api/client/api";

const {
    servicesGetMock,
    servicesPostMock,
    servicesDeleteMock,
    profileGetMock,
} = vi.hoisted(() => ({
    servicesGetMock: vi.fn(),
    servicesPostMock: vi.fn(),
    servicesDeleteMock: vi.fn(),
    profileGetMock: vi.fn(),
}));

vi.mock("@/api/api", () => ({
    __esModule: true,
    default: {
        Client: {
            servicesApi: {
                apiV1ServicesGet: servicesGetMock,
            },
            profilesApi: {
                apiV1ProfilesIdServicesPost: servicesPostMock,
                apiV1ProfilesIdServicesDelete: servicesDeleteMock,
                apiV1ProfilesIdGet: profileGetMock,
            },
        },
    },
}));

vi.mock("@/components/ui/scroll-area", () => ({
    __esModule: true,
    ScrollArea: ({ children }: { children: React.ReactNode }) => (
        <div data-testid="scroll-area">{children}</div>
    ),
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

const makeProfile = (blocked: string[] = []) =>
    ({
        profile_id: "profile-1",
        id: "profile-1",
        name: "Primary",
        account_id: "account-1",
        settings: {
            privacy: {
                services: blocked,
            },
        },
    }) as unknown as ModelProfile;

describe("ServicesContentSection", () => {
    beforeEach(() => {
        servicesGetMock.mockReset();
        servicesPostMock.mockReset();
        servicesDeleteMock.mockReset();
        profileGetMock.mockReset();

        useAppStore.setState({ activeProfile: makeProfile([]) });

        servicesGetMock.mockResolvedValue({
            data: {
                services: [
                    {
                        id: "google",
                        name: "Google",
                        asns: [1, 2, 3, 4, 5, 6],
                    },
                ],
            },
        });

        servicesPostMock.mockResolvedValue({ status: 200 });
        servicesDeleteMock.mockResolvedValue({ status: 200 });
    });

    afterEach(() => {
        useAppStore.setState({ activeProfile: null });
    });

    it("fetches catalog and toggles service block/unblock", async () => {
        profileGetMock
            .mockResolvedValueOnce({ data: makeProfile(["google"]) })
            .mockResolvedValueOnce({ data: makeProfile([]) });

        const user = userEvent.setup();
        render(<ServicesContentSection />);

        await screen.findByText("Google");

        expect(screen.getByRole("img", { name: /google logo/i })).toBeInTheDocument();

        const asnsEl = screen.getByTestId("service-asns");
        expect(asnsEl).toHaveTextContent("ASN: 1, 2, 3, 4, 5 +1");
        expect(asnsEl).toHaveAttribute("title", "ASN: 1, 2, 3, 4, 5, 6");

        const switchEl = screen.getByRole("switch");
        expect(switchEl).toHaveAttribute("aria-checked", "false");

        await user.click(switchEl);

        await waitFor(() => {
            expect(servicesPostMock).toHaveBeenCalledWith("profile-1", {
                service_ids: ["google"],
            });
        });

        await waitFor(() => {
            expect(profileGetMock).toHaveBeenCalledWith("profile-1");
            expect(switchEl).toHaveAttribute("aria-checked", "true");
        });

        await user.click(switchEl);

        await waitFor(() => {
            expect(servicesDeleteMock).toHaveBeenCalledWith("profile-1", {
                service_ids: ["google"],
            });
        });

        await waitFor(() => {
            expect(switchEl).toHaveAttribute("aria-checked", "false");
        });
    });

    it("does nothing when no active profile is selected", async () => {
        useAppStore.setState({ activeProfile: null });

        const user = userEvent.setup();
        render(<ServicesContentSection />);

        await screen.findByText("Google");

        const switchEl = screen.getByRole("switch");
        await user.click(switchEl);

        expect(servicesPostMock).not.toHaveBeenCalled();
        expect(servicesDeleteMock).not.toHaveBeenCalled();
        expect(profileGetMock).not.toHaveBeenCalled();
    });
});
