import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { ModelProfile } from '@/api/client/api';

const STORAGE_KEY = 'moddns-storage';

const profile = (id: string, profileId: string): ModelProfile => ({
    id,
    profile_id: profileId,
    account_id: 'acc-1',
    name: `Profile ${id}`,
    settings: {
        custom_rules: [{ id: 'r1', action: 'block', value: 'example.com', notes: 'private note' }],
    },
} as unknown as ModelProfile);

function stored(): Record<string, unknown> {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw).state : {};
}

// The store hydrates from localStorage at module load, so each test imports a
// fresh module graph after seeding storage.
async function loadStore() {
    vi.resetModules();
    return (await import('@/store/general')).useAppStore;
}

describe('app store persistence', () => {
    beforeEach(() => {
        localStorage.clear();
    });

    it('persists the selected profile id but never the profile object', async () => {
        const useAppStore = await loadStore();
        const p = profile('mongo-1', 'dnscred1');
        useAppStore.getState().setProfiles([p]);
        useAppStore.getState().setActiveProfile(p);

        const state = stored();
        expect(state.activeProfile).toBeUndefined();
        expect(state.activeProfileId).toBe('mongo-1');
        expect(localStorage.getItem(STORAGE_KEY)).not.toContain('dnscred1');
        expect(localStorage.getItem(STORAGE_KEY)).not.toContain('private note');
        expect(localStorage.getItem(STORAGE_KEY)).not.toContain('acc-1');
    });

    it('restores the selected profile by id from the fetched list after a reload', async () => {
        let useAppStore = await loadStore();
        const a = profile('mongo-a', 'creda');
        const b = profile('mongo-b', 'credb');
        useAppStore.getState().setProfiles([a, b]);
        useAppStore.getState().setActiveProfile(b);

        useAppStore = await loadStore();
        expect(useAppStore.getState().activeProfile).toBeNull();
        useAppStore.getState().restoreActiveProfile([a, b]);
        expect(useAppStore.getState().activeProfile?.id).toBe('mongo-b');
    });

    it('migrates a legacy stored profile object to its id and drops it from storage', async () => {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({
            version: 0,
            state: {
                activeProfile: profile('mongo-legacy', 'legacycred'),
                customRulesCollapsed: { 'legacycred:denied': ['work'] },
                verificationBannerDismissed: true,
                connectionStatusVisible: false,
            },
        }));

        const useAppStore = await loadStore();
        const s = useAppStore.getState();
        expect(s.activeProfile).toBeNull();
        expect(s.activeProfileId).toBe('mongo-legacy');
        expect(s.customRulesCollapsed).toEqual({});
        expect(s.verificationBannerDismissed).toBe(true);
        expect(s.connectionStatusVisible).toBe(false);

        // Any write after hydration rewrites storage in the new shape.
        useAppStore.getState().setRightPanelOpen(true);
        expect(localStorage.getItem(STORAGE_KEY)).not.toContain('legacycred');
    });

    it('resetAccountState clears account data and keeps device preferences and dismissals', async () => {
        const useAppStore = await loadStore();
        const p = profile('mongo-1', 'dnscred1');
        const st = useAppStore.getState();
        st.setProfiles([p]);
        st.setActiveProfile(p);
        st.setAccount({ id: 'acc-1', email: 'a@example.com' } as never);
        st.setPasskeys([{ id: 'pk' } as never]);
        st.setSubscriptionStatus('active');
        st.setSubscriptionType('Managed');
        st.setSubscriptionDeletionScheduled(true);
        st.setCustomRulesCollapsed('mongo-1:denied', ['work']);
        st.setConnectionStatusVisible(false);
        st.setVerificationBannerDismissed(true);
        st.setBlocklistsAlertDismissed(true);
        st.setCustomRulesAlertDismissed(true);
        st.setLogsExpandHintDismissed(true);
        st.markAnnouncementsSeen();

        useAppStore.getState().resetAccountState();

        const s = useAppStore.getState();
        expect(s.activeProfile).toBeNull();
        expect(s.activeProfileId).toBeNull();
        expect(s.profiles).toEqual([]);
        expect(s.account).toBeNull();
        expect(s.passkeys).toEqual([]);
        expect(s.subscriptionStatus).toBeNull();
        expect(s.subscriptionType).toBeNull();
        expect(s.subscriptionDeletionScheduled).toBe(false);
        expect(s.customRulesCollapsed).toEqual({});

        expect(s.connectionStatusVisible).toBe(false);
        expect(s.verificationBannerDismissed).toBe(true);
        expect(s.blocklistsAlertDismissed).toBe(true);
        expect(s.customRulesAlertDismissed).toBe(true);
        expect(s.logsExpandHintDismissed).toBe(true);
        expect(s.announcementsLastSeenAt).not.toBeNull();

        const raw = localStorage.getItem(STORAGE_KEY) ?? '';
        expect(raw).not.toContain('mongo-1');
        expect(raw).not.toContain('work');
        expect(stored().activeProfileId).toBeNull();
    });
});
