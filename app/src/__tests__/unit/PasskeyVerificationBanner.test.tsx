import { render, screen, act, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import PasskeyVerificationBanner from '@/components/PasskeyVerificationBanner';
import { useLoadPasskeysOnce } from '@/hooks/useLoadPasskeysOnce';
import { AUTH_KEY } from '@/lib/consts';

const deferred = <T,>() => { let resolve!: (v: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; };

function LayoutWithBanner({ signedIn = true }: { signedIn?: boolean }) {
    useLoadPasskeysOnce(signedIn);
    return <PasskeyVerificationBanner />;
}

function renderBanner(signedIn = true) {
    return render(<MemoryRouter><LayoutWithBanner signedIn={signedIn} /></MemoryRouter>);
}

// specRef: api-endpoint-behaviour F16
describe('PasskeyVerificationBanner', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
        useAppStore.getState().resetAccountState();
        localStorage.setItem(AUTH_KEY, 'true');
    });

    it('shows the deadline once a passkey reports no user verification', async () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['passkey'] } as never);
        vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockResolvedValue({ data: [{ id: 'p1', user_verified: false }] } as never);

        renderBanner();

        const banner = await screen.findByTestId('passkey-verification-banner');
        expect(banner).toHaveTextContent('One of your passkeys needs an important update.');
        expect(banner).toHaveTextContent("From December 31, 2026, signing in with a passkey will ask for your fingerprint, face, device password or security key PIN, and a passkey that can't will stop working.");
        expect(screen.getByRole('link', { name: 'See what to do' })).toHaveAttribute('href', '/faq#passkey-pin');
    });

    it('shows nothing for an account whose passkeys all verify the user', async () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['password', 'passkey'] } as never);
        const get = vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockResolvedValue({ data: [{ id: 'p1', user_verified: true }] } as never);

        renderBanner();

        await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
    });

    it('does not load passkeys for an account known to have none', () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['password'] } as never);
        const get = vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet');

        renderBanner();

        expect(get).not.toHaveBeenCalled();
    });

    it('does not load passkeys while signed out', () => {
        const get = vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet');

        renderBanner(false);
        localStorage.removeItem(AUTH_KEY);
        renderBanner(true);

        expect(get).not.toHaveBeenCalled();
    });

    it('loads the list when the route did not load the account', async () => {
        vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockResolvedValue({ data: [{ id: 'p1', user_verified: false }] } as never);

        renderBanner();

        expect(await screen.findByTestId('passkey-verification-banner')).toHaveTextContent('One of your passkeys needs an important update.');
    });

    it('drops a passkey list that arrives after logout', async () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['passkey'] } as never);
        const pending = deferred<unknown>();
        vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockReturnValue(pending.promise as never);

        renderBanner();
        act(() => { useAppStore.getState().resetAccountState(); });
        await act(async () => { pending.resolve({ data: [{ id: 'p1', user_verified: false }] }); await Promise.resolve(); });

        expect(useAppStore.getState().passkeys).toEqual([]);
    });

    it('keeps a newer empty list after the last passkey was deleted', async () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['passkey'] } as never);
        const pending = deferred<unknown>();
        vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockReturnValue(pending.promise as never);

        renderBanner();
        act(() => { useAppStore.getState().setPasskeys([]); });
        await act(async () => { pending.resolve({ data: [{ id: 'p1', user_verified: false }] }); await Promise.resolve(); });

        expect(useAppStore.getState().passkeys).toEqual([]);
    });

    it('keeps a newer list loaded by Passkey settings', async () => {
        useAppStore.getState().setAccount({ id: 'a1', auth_methods: ['passkey'] } as never);
        const pending = deferred<unknown>();
        vi.spyOn(api.Client.authApi, 'apiV1WebauthnPasskeysGet').mockReturnValue(pending.promise as never);

        renderBanner();
        act(() => { useAppStore.getState().setPasskeys([{ id: 'p2', user_verified: true }]); });
        await act(async () => { pending.resolve({ data: [{ id: 'p1', user_verified: false }] }); await Promise.resolve(); });

        expect(useAppStore.getState().passkeys).toEqual([{ id: 'p2', user_verified: true }]);
    });
});
