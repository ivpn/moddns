import { useContext } from 'react';
import { render, screen, act, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import type { ModelProfile } from '@/api/client/api';
import { AuthContext, AuthProvider, rootLoader, profilesOnlyLoader } from '@/App';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import { AUTH_KEY } from '@/lib/consts';
import { dispatch } from '@/lib/eventBus';

const navigateMock = vi.fn();

vi.mock('react-router-dom', async () => {
    const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
    return { ...actual, useNavigate: () => navigateMock };
});

const STORAGE_KEY = 'moddns-storage';
const PROFILE = { id: 'mongo-1', profile_id: 'dnscred1', name: 'Home' } as ModelProfile;

function Consumer() {
    const auth = useContext(AuthContext);
    return (
        <>
            <span data-testid="authed">{String(auth?.isAuthenticated)}</span>
            <button onClick={() => auth?.logout()}>logout</button>
            <button onClick={() => auth?.login(false)}>login</button>
        </>
    );
}

function renderProvider() {
    return render(
        <MemoryRouter initialEntries={['/home']}>
            <AuthProvider>
                <Consumer />
            </AuthProvider>
        </MemoryRouter>,
    );
}

function seedSignedIn() {
    localStorage.setItem(AUTH_KEY, 'true');
    const st = useAppStore.getState();
    st.setProfiles([PROFILE]);
    st.setActiveProfile(PROFILE);
    st.setCustomRulesCollapsed('mongo-1:denied', ['work']);
    st.setPasskeys([{ id: 'pk' } as never]);
    st.setSubscriptionDeletionScheduled(true);
}

function expectAccountStateCleared() {
    const s = useAppStore.getState();
    expect(s.activeProfile).toBeNull();
    expect(s.activeProfileId).toBeNull();
    expect(s.profiles).toEqual([]);
    expect(s.passkeys).toEqual([]);
    expect(s.subscriptionDeletionScheduled).toBe(false);
    expect(s.customRulesCollapsed).toEqual({});
    const raw = localStorage.getItem(STORAGE_KEY) ?? '';
    expect(raw).not.toContain('mongo-1');
    expect(raw).not.toContain('work');
}

describe('AuthProvider clears account state from the browser', () => {
    beforeEach(() => {
        localStorage.clear();
        navigateMock.mockReset();
        useAppStore.getState().resetAccountState();
    });

    it('on explicit logout', () => {
        seedSignedIn();
        renderProvider();
        fireEvent.click(screen.getByText('logout'));
        expectAccountStateCleared();
    });

    it('on session expiry', () => {
        seedSignedIn();
        renderProvider();
        act(() => { dispatch({ type: 'auth/sessionExpired' }); });
        expectAccountStateCleared();
    });

    it('when another tab logs out', () => {
        seedSignedIn();
        renderProvider();
        expect(screen.getByTestId('authed')).toHaveTextContent('true');

        act(() => {
            localStorage.removeItem(AUTH_KEY);
            window.dispatchEvent(new StorageEvent('storage', { key: AUTH_KEY, oldValue: 'true', newValue: null }));
        });

        expect(screen.getByTestId('authed')).toHaveTextContent('false');
        expectAccountStateCleared();
    });

    it('ignores storage events for other keys', () => {
        seedSignedIn();
        renderProvider();
        act(() => {
            window.dispatchEvent(new StorageEvent('storage', { key: 'vite-ui-theme', newValue: 'light' }));
        });
        expect(screen.getByTestId('authed')).toHaveTextContent('true');
        expect(useAppStore.getState().activeProfile?.id).toBe('mongo-1');
    });
});

describe('AuthProvider login', () => {
    beforeEach(() => {
        localStorage.clear();
        useAppStore.getState().resetAccountState();
    });

    it('starts from an empty account store', () => {
        seedSignedIn();
        useAppStore.getState().setAccount({ id: 'acc-1' } as never);
        localStorage.removeItem(AUTH_KEY);
        renderProvider();

        fireEvent.click(screen.getByText('login'));

        expect(useAppStore.getState().account).toBeNull();
        expectAccountStateCleared();
    });
});

describe('background refresh after logout', () => {
    const deferred = <T,>() => { let resolve!: (v: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; };
    const flush = () => act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve(); });

    beforeEach(() => {
        localStorage.clear();
        useAppStore.getState().resetAccountState();
        vi.restoreAllMocks();
    });

    function seedWarmCache() {
        seedSignedIn();
        useAppStore.getState().setAccount({ id: 'acc-1' } as never);
    }

    function logoutElsewhere() {
        localStorage.removeItem(AUTH_KEY);
        useAppStore.getState().resetAccountState();
    }

    it('rootLoader drops account and profiles that arrive after logout', async () => {
        seedWarmCache();
        const account = deferred<unknown>();
        const profiles = deferred<unknown>();
        vi.spyOn(api.Client.accountsApi, 'apiV1AccountsCurrentGet').mockReturnValue(account.promise as never);
        vi.spyOn(api.Client.profilesApi, 'apiV1ProfilesGet').mockReturnValue(profiles.promise as never);

        await rootLoader();
        logoutElsewhere();
        account.resolve({ data: { id: 'acc-1' } });
        profiles.resolve({ data: [PROFILE] });
        await flush();

        expect(useAppStore.getState().account).toBeNull();
        expectAccountStateCleared();
    });

    it('profilesOnlyLoader drops profiles that arrive after logout', async () => {
        seedWarmCache();
        const profiles = deferred<unknown>();
        vi.spyOn(api.Client.profilesApi, 'apiV1ProfilesGet').mockReturnValue(profiles.promise as never);

        await profilesOnlyLoader();
        logoutElsewhere();
        profiles.resolve({ data: [PROFILE] });
        await flush();

        expectAccountStateCleared();
    });
});
