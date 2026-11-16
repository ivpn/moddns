import { render, screen, act, waitFor, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import PasskeyVerificationBanner from '@/components/PasskeyVerificationBanner';
import { useLoadPasskeysOnce } from '@/hooks/useLoadPasskeysOnce';
import { AUTH_KEY } from '@/lib/consts';
import { PASSKEY_BANNER_DISMISSED_KEY, clearPasskeyBannerDismissal } from '@/lib/passkeyVerification';

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
        localStorage.clear();
        localStorage.setItem(AUTH_KEY, 'true');
        clearPasskeyBannerDismissal();
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

// specRef: api-endpoint-behaviour F17
describe('PasskeyVerificationBanner dismissal', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
        useAppStore.getState().resetAccountState();
        localStorage.clear();
        localStorage.setItem(AUTH_KEY, 'true');
        clearPasskeyBannerDismissal();
    });

    const setUp = (authMethods: string[] | null, passkeys: { id?: string; user_verified?: boolean }[]) => {
        if (authMethods) useAppStore.getState().setAccount({ id: 'a1', auth_methods: authMethods } as never);
        useAppStore.getState().setPasskeys(passkeys);
    };
    const renderStatic = () => render(<MemoryRouter><PasskeyVerificationBanner /></MemoryRouter>);
    const close = () => screen.queryByRole('button', { name: 'Close' });

    it('can be closed by an account with a password, and stays closed on the next page', () => {
        setUp(['password', 'passkey'], [{ id: 'p1', user_verified: false }]);

        const { unmount } = renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toHaveTextContent('You can still sign in with your password.');
        fireEvent.click(close()!);
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
        expect(localStorage.getItem(PASSKEY_BANNER_DISMISSED_KEY)).toBe('p1');

        unmount();
        renderStatic();
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
    });

    it('can be closed by an account with another verified passkey', () => {
        setUp(['passkey'], [{ id: 'p1', user_verified: false }, { id: 'p2', user_verified: true }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toHaveTextContent('You can still sign in with your other passkey.');
        fireEvent.click(close()!);
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
    });

    it('cannot be closed when only unverified passkeys can sign in', () => {
        setUp(['passkey'], [{ id: 'p1', user_verified: false }, { id: 'p2' }]);

        renderStatic();
        const banner = screen.getByTestId('passkey-verification-banner');
        expect(banner).not.toHaveTextContent('You can still sign in');
        expect(close()).toBeNull();
    });

    it('cannot be closed while the account is not loaded', () => {
        setUp(null, [{ id: 'p1', user_verified: false }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toBeInTheDocument();
        expect(close()).toBeNull();
    });

    it('keeps a stored dismissal while the account is not loaded', () => {
        localStorage.setItem(PASSKEY_BANNER_DISMISSED_KEY, 'p1');
        setUp(null, [{ id: 'p1', user_verified: false }]);

        renderStatic();
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
    });

    it('can be closed before the account loads when another passkey verifies the user', () => {
        setUp(null, [{ id: 'p1', user_verified: false }, { id: 'p2', user_verified: true }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toHaveTextContent('You can still sign in with your other passkey.');
        expect(close()).not.toBeNull();
    });

    it('forgets the dismissal when cleared on logout', () => {
        setUp(['password', 'passkey'], [{ id: 'p1', user_verified: false }]);
        const { unmount } = renderStatic();
        fireEvent.click(close()!);
        unmount();

        clearPasskeyBannerDismissal();

        expect(localStorage.getItem(PASSKEY_BANNER_DISMISSED_KEY)).toBeNull();
        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toBeInTheDocument();
    });

    it('ignores a stored dismissal once the account can no longer sign in another way', () => {
        localStorage.setItem(PASSKEY_BANNER_DISMISSED_KEY, 'p1');
        setUp(['passkey'], [{ id: 'p1', user_verified: false }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toBeInTheDocument();
        expect(close()).toBeNull();
    });

    it('shows again when another passkey becomes unverified', () => {
        localStorage.setItem(PASSKEY_BANNER_DISMISSED_KEY, 'p1');
        setUp(['password', 'passkey'], [{ id: 'p1', user_verified: false }, { id: 'p3', user_verified: false }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toBeInTheDocument();
        fireEvent.click(close()!);
        expect(localStorage.getItem(PASSKEY_BANNER_DISMISSED_KEY)).toBe('p1,p3');
    });

    it('cannot be closed when an unverified passkey has no id', () => {
        setUp(['password', 'passkey'], [{ id: 'p1', user_verified: false }, { user_verified: false }]);

        renderStatic();
        expect(screen.getByTestId('passkey-verification-banner')).toBeInTheDocument();
        expect(close()).toBeNull();
    });

    it('still closes, until reload, when storage throws', () => {
        setUp(['password', 'passkey'], [{ id: 'p1', user_verified: false }]);
        vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied'); });
        vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied'); });

        const { unmount } = renderStatic();
        fireEvent.click(close()!);
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();

        unmount();
        renderStatic();
        expect(screen.queryByTestId('passkey-verification-banner')).toBeNull();
    });
});
