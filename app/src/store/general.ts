import type { ModelAccount, ModelCredential, ModelProfile } from "@/api/client/api";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { useMemo } from "react";

interface AppState {
  activeProfile: ModelProfile | null;
  // Internal profile id (not the DNS credential `profile_id`); the only profile
  // data kept in browser storage, used to reselect the profile after a reload.
  activeProfileId: string | null;
  setActiveProfile: (profile: ModelProfile | null) => void;
  profiles: ModelProfile[];
  setProfiles: (profiles: ModelProfile[]) => void;
  restoreActiveProfile: (profiles: ModelProfile[]) => void;
  account: ModelAccount | null;
  setAccount: (account: ModelAccount | null) => void;
  rightPanelOpen: boolean;
  setRightPanelOpen: (isOpen: boolean) => void;
  connectionStatusVisible: boolean;
  setConnectionStatusVisible: (isVisible: boolean) => void;
  verificationBannerDismissed: boolean; // persisted dismissal of email verification banner
  setVerificationBannerDismissed: (dismissed: boolean) => void;
  blocklistsAlertDismissed: boolean; // persisted dismissal of the blocklists alert
  setBlocklistsAlertDismissed: (dismissed: boolean) => void;
  customRulesAlertDismissed: boolean; // persisted dismissal of the custom rules alert
  setCustomRulesAlertDismissed: (dismissed: boolean) => void;
  logsExpandHintDismissed: boolean; // persisted dismissal of the one-time "tap a row" logs hint
  setLogsExpandHintDismissed: (dismissed: boolean) => void;
  passkeys: ModelCredential[];
  setPasskeys: (passkeys: ModelCredential[]) => void;
  subscriptionStatus: string | null;
  setSubscriptionStatus: (status: string | null) => void;
  // Legacy pre-0.1.8 enum surfaced from the API; "Managed" gates the
  // beta-ending banner. Cleared to "" by the backend after a successful resync.
  subscriptionType: string | null;
  setSubscriptionType: (type: string | null) => void;
  // True when the account was retired by the signup-reset flow (status
  // pending_delete). Distinguishes a retired account from a normal `inactive`
  // account so the webapp can show "this account was replaced" copy instead of
  // "add time to recover".
  subscriptionDeletionScheduled: boolean;
  setSubscriptionDeletionScheduled: (scheduled: boolean) => void;
  // ISO timestamp of when the user last opened the Announcements page; used to
  // compute the unread indicator in the nav. null = never opened.
  announcementsLastSeenAt: string | null;
  markAnnouncementsSeen: () => void;
  customRulesCollapsed: Record<string, string[]>;
  setCustomRulesCollapsed: (key: string, collapsedNames: string[]) => void;
  // Drops everything tied to the signed-in account; device preferences and
  // dismissals stay.
  resetAccountState: () => void;
  // Bumped by every reset; a background fetch that started under an older
  // epoch must not write its result.
  authEpoch: number;
}

const accountState = {
  activeProfile: null,
  activeProfileId: null,
  profiles: [],
  account: null,
  passkeys: [],
  subscriptionStatus: null,
  subscriptionType: null,
  subscriptionDeletionScheduled: false,
  customRulesCollapsed: {},
} satisfies Partial<AppState>;

export const useAppStore = create<AppState>()(
  persist(
    (set, get) => ({
      ...accountState,
      // Mutations store the refetched profile here, so mirror it into `profiles`;
      // profile switching and the account-wide rule count read from that list.
      setActiveProfile: (profile) => {
        set((s) => ({
          activeProfile: profile,
          activeProfileId: profile?.id ?? null,
          profiles: profile
            ? s.profiles.map((p) => (p.profile_id === profile.profile_id ? profile : p))
            : s.profiles,
        }));
      },
      setProfiles: (profiles) => set({ profiles }),
      restoreActiveProfile: (profiles) => {
        const currentId = get().activeProfile?.id ?? get().activeProfileId;
        if (currentId && profiles.length > 0) {
          // Try to find the current active profile in the new profiles list
          const foundProfile = profiles.find(p => p.id === currentId);
          if (foundProfile) {
            set({ activeProfile: foundProfile, activeProfileId: foundProfile.id });
            return;
          }
        }
        // If no active profile or profile not found, set the first one
        if (profiles.length > 0) {
          set({ activeProfile: profiles[0], activeProfileId: profiles[0].id });
        }
      },
      setAccount: (account) => set({ account }),
      rightPanelOpen: false,
      setRightPanelOpen: (isOpen) => set({ rightPanelOpen: isOpen }),
      connectionStatusVisible: true,
      setConnectionStatusVisible: (isVisible) => set({ connectionStatusVisible: isVisible }),
      verificationBannerDismissed: false,
      setVerificationBannerDismissed: (dismissed) => set({ verificationBannerDismissed: dismissed }),
      blocklistsAlertDismissed: false,
      setBlocklistsAlertDismissed: (dismissed) => set({ blocklistsAlertDismissed: dismissed }),
      customRulesAlertDismissed: false,
      setCustomRulesAlertDismissed: (dismissed) => set({ customRulesAlertDismissed: dismissed }),
      logsExpandHintDismissed: false,
      setLogsExpandHintDismissed: (dismissed) => set({ logsExpandHintDismissed: dismissed }),
      setPasskeys: (passkeys) => set({ passkeys }),
      setSubscriptionStatus: (status) => set({ subscriptionStatus: status }),
      setSubscriptionType: (type) => set({ subscriptionType: type }),
      setSubscriptionDeletionScheduled: (scheduled) => set({ subscriptionDeletionScheduled: scheduled }),
      announcementsLastSeenAt: null,
      markAnnouncementsSeen: () => set({ announcementsLastSeenAt: new Date().toISOString() }),
      setCustomRulesCollapsed: (key, collapsedNames) =>
        set((s) => ({ customRulesCollapsed: { ...s.customRulesCollapsed, [key]: collapsedNames } })),
      authEpoch: 0,
      resetAccountState: () => set((s) => ({ ...structuredClone(accountState), authEpoch: s.authEpoch + 1 })),
    }),
    {
      name: "moddns-storage",
      version: 1,
      // v0 payloads carry the whole active profile and collapse keys built from
      // the DNS credential; keep only the profile's internal id.
      migrate: (persisted, version) => {
        const state = (persisted ?? {}) as Partial<AppState> & { activeProfile?: ModelProfile | null };
        if (version < 1) {
          const { activeProfile, ...rest } = state;
          return { ...rest, activeProfileId: activeProfile?.id ?? null, customRulesCollapsed: {} } as AppState;
        }
        return state as AppState;
      },
      // Persist only the selected profile id and lightweight UI state
      partialize: (state) => ({
        activeProfileId: state.activeProfile?.id ?? state.activeProfileId,
        verificationBannerDismissed: state.verificationBannerDismissed,
        blocklistsAlertDismissed: state.blocklistsAlertDismissed,
        customRulesAlertDismissed: state.customRulesAlertDismissed,
        connectionStatusVisible: state.connectionStatusVisible,
        announcementsLastSeenAt: state.announcementsLastSeenAt,
        customRulesCollapsed: state.customRulesCollapsed,
        logsExpandHintDismissed: state.logsExpandHintDismissed,
      }),
    }
  )
);

// Centralized profile data shape consumed by setup guides & UI
export interface DerivedProfileData {
  id: string;                // profile id
  dnsOverTLS: string;        // <profileId>.<domain>
  dnsOverHTTPS: string;      // https://<domain>/dns-query/<profileId>
  ipv4: string;              // primary IPv4 (first in env list)
  ipv6?: string;             // placeholder / future real value
  domain: string;            // DNS server domain
  dohEndpoint: string;       // alias for dnsOverHTTPS (convenience)
}

// Hook: returns memoized derived profile data or null if no active profile
export function useProfileData(): DerivedProfileData | null {
  const activeProfile = useAppStore(s => s.activeProfile);
  // Access Vite env (runtime safe)
  const envDomain = (import.meta as ImportMeta).env.VITE_DNS_SERVER_DOMAIN || 'example.com';
  const rawIps: string = (import.meta as ImportMeta).env.VITE_DNS_SERVER_IP_ADDRESSES || '';
  const primaryIp = rawIps.split(',').map((s: string) => s.trim()).filter(Boolean)[0] || '0.0.0.0';
  const rawIpv6: string = (import.meta as ImportMeta).env.VITE_DNS_SERVER_IPV6_ADDRESSES || '';
  const primaryIpv6 = rawIpv6.split(',').map((s: string) => s.trim()).filter(Boolean)[0] || undefined;

  return useMemo(() => {
    if (!activeProfile) return null;
    const id = (activeProfile as ModelProfile & { profile_id?: string; id?: string }).profile_id || (activeProfile as ModelProfile & { id?: string }).id || '';
    const dnsOverTLS = `${id}.${envDomain}`;
    const dnsOverHTTPS = `https://${envDomain}/dns-query/${id}`;
    return {
      id,
      dnsOverTLS,
      dnsOverHTTPS,
      ipv4: primaryIp,
      ipv6: primaryIpv6, // anycast IPv6 from env; undefined when not configured (e.g. staging)
      domain: envDomain,
      dohEndpoint: dnsOverHTTPS,
    } as DerivedProfileData;
  }, [activeProfile, envDomain, primaryIp, primaryIpv6]);
}
