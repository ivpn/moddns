// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { MemoryRouter, RouterProvider, createMemoryRouter, useLocation } from 'react-router-dom';
import AccountSubscription from '@/components/AccountSubscription';
import api from '@/api/api';

vi.mock('sonner', () => ({
    toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            paSessionApi: { apiV1PasessionRotatePut: vi.fn() },
            subscriptionApi: { apiV1SubGet: vi.fn(), apiV1SubUpdatePut: vi.fn() },
        },
    },
}));

type GenericMock = ReturnType<typeof vi.fn>;
const rotateMock = api.Client.paSessionApi.apiV1PasessionRotatePut as unknown as GenericMock;
const subGetMock = api.Client.subscriptionApi.apiV1SubGet as unknown as GenericMock;
const subUpdateMock = api.Client.subscriptionApi.apiV1SubUpdatePut as unknown as GenericMock;

// Synced links are remembered for the page load, so each test uses its own id.
const SESSION_ID = '0f8fad5b-d9cb-469f-a165-70867728950e';
const sessionId = (n: number) => `0f8fad5b-d9cb-469f-a165-7086772895${String(n).padStart(2, '0')}`;
const SUB_ID = '7c9e6679-7425-40de-944b-e07fc1f90ae7';

const LocationProbe = () => {
    const location = useLocation();
    return <div data-testid="search">{location.search}</div>;
};

const renderAt = (url: string, strict = false) => {
    const tree = (
        <MemoryRouter initialEntries={[url]}>
            <AccountSubscription />
            <LocationProbe />
        </MemoryRouter>
    );
    render(strict ? <StrictMode>{tree}</StrictMode> : tree);
};

const search = () => screen.getByTestId('search').textContent ?? '';

describe('AccountSubscription resync link', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        subGetMock.mockResolvedValue({ data: { status: 'active', active_until: '2027-01-01T00:00:00Z' } });
        rotateMock.mockResolvedValue({});
        subUpdateMock.mockResolvedValue({});
    });

    // specRef: SY16
    it('removes the one-time link parameters after a successful sync and keeps others', async () => {
        renderAt(`/account-preferences?sessionid=${SESSION_ID}&subid=${SUB_ID}&tab=x`);

        await waitFor(() => expect(subUpdateMock).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(search()).toBe('?tab=x'));
        expect(rotateMock).toHaveBeenCalledWith({ sessionid: SESSION_ID });
        expect(subUpdateMock).toHaveBeenCalledWith({ subid: SUB_ID });
    });

    // specRef: SY16
    it('syncs once per link even when effects run twice', async () => {
        renderAt(`/account-preferences?sessionid=${sessionId(1)}`, true);

        await waitFor(() => expect(search()).toBe(''));
        expect(rotateMock).toHaveBeenCalledTimes(1);
        expect(subUpdateMock).toHaveBeenCalledTimes(1);
    });

    // specRef: SY16
    it('keeps the link parameters after a retryable failure so a reload retries', async () => {
        subUpdateMock.mockRejectedValue({ response: { status: 400 } });
        const id = sessionId(2);
        renderAt(`/account-preferences?sessionid=${id}`);

        expect(await screen.findByText('Failed to sync subscription. Please try again.')).toBeTruthy();
        expect(search()).toBe(`?sessionid=${id}`);

        cleanup();
        subUpdateMock.mockResolvedValue({});
        renderAt(`/account-preferences?sessionid=${id}`);
        await waitFor(() => expect(search()).toBe(''));
        expect(subUpdateMock).toHaveBeenCalledTimes(2);
    });

    // specRef: SY16
    it('removes the link parameters when the account can no longer be synced', async () => {
        subUpdateMock.mockRejectedValue({ response: { status: 409 } });
        renderAt(`/account-preferences?sessionid=${sessionId(3)}`);

        expect(await screen.findByText(/can no longer be synced/)).toBeTruthy();
        await waitFor(() => expect(search()).toBe(''));
    });

    // specRef: SY16
    it('shows the expired message and removes the link parameters when rotate refuses the link', async () => {
        rotateMock.mockRejectedValue({ response: { status: 400 } });
        const id = sessionId(5);
        renderAt(`/account-preferences?sessionid=${id}&subid=${SUB_ID}`);

        expect(await screen.findByText(/This signup link has expired/)).toBeTruthy();
        await waitFor(() => expect(search()).toBe(''));
        expect(subUpdateMock).not.toHaveBeenCalled();

        cleanup();
        renderAt(`/account-preferences?sessionid=${id}`);
        await waitFor(() => expect(subGetMock).toHaveBeenCalledTimes(2));
        expect(rotateMock).toHaveBeenCalledTimes(1);
    });

    // specRef: SY16
    it('keeps the link parameters when rotate fails for another reason', async () => {
        rotateMock.mockRejectedValue({ response: { status: 500 } });
        const id = sessionId(6);
        renderAt(`/account-preferences?sessionid=${id}`);

        expect(await screen.findByText('Failed to sync subscription. Please try again.')).toBeTruthy();
        expect(search()).toBe(`?sessionid=${id}`);
    });

    // specRef: SY16
    it('does not navigate after unmount or sync the same link again on remount', async () => {
        let finish: (v: unknown) => void = () => {};
        subUpdateMock.mockReturnValue(new Promise(r => { finish = r; }));
        const id = sessionId(4);
        const router = createMemoryRouter(
            [
                { path: '/account-preferences', element: <><AccountSubscription /><LocationProbe /></> },
                { path: '/profiles', element: <LocationProbe /> },
            ],
            { initialEntries: [`/account-preferences?sessionid=${id}`] },
        );
        render(<RouterProvider router={router} />);
        await waitFor(() => expect(subUpdateMock).toHaveBeenCalledTimes(1));

        await act(async () => { await router.navigate('/profiles'); });
        await act(async () => { finish({}); });
        expect(router.state.location.pathname).toBe('/profiles');

        await act(async () => { await router.navigate(-1); });
        expect(router.state.location.pathname).toBe('/account-preferences');
        await waitFor(() => expect(subGetMock).toHaveBeenCalled());
        expect(rotateMock).toHaveBeenCalledTimes(1);
        expect(subUpdateMock).toHaveBeenCalledTimes(1);
    });
});
