import type { ModelAccount, ModelCredential, ModelProfile } from "@/api/client/api";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { useMemo } from "react";

interface AppState {
  activeProfile: ModelProfile | null;
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
  passkeys: ModelCredential[];
  setPasskeys: (passkeys: ModelCredential[]) => void;
}

export const useAppStore = create<AppState>()(
  persist(
    (set, get) => ({
      activeProfile: null,
      setActiveProfile: (profile) => {
        set({ activeProfile: profile });
      },
      profiles: [],
      setProfiles: (profiles) => set({ profiles }),
      restoreActiveProfile: (profiles) => {
        const currentActiveProfile = get().activeProfile;
        if (currentActiveProfile && profiles.length > 0) {
          // Try to find the current active profile in the new profiles list
          const foundProfile = profiles.find(p => p.id === currentActiveProfile.id);
          if (foundProfile) {
            set({ activeProfile: foundProfile });
            return;
          }
        }
        // If no active profile or profile not found, set the first one
        if (profiles.length > 0) {
          set({ activeProfile: profiles[0] });
        }
      },
      account: null,
      setAccount: (account) => set({ account }),
      rightPanelOpen: false,
      setRightPanelOpen: (isOpen) => set({ rightPanelOpen: isOpen }),
      connectionStatusVisible: true,
      setConnectionStatusVisible: (isVisible) => set({ connectionStatusVisible: isVisible }),
      verificationBannerDismissed: false,
      setVerificationBannerDismissed: (dismissed) => set({ verificationBannerDismissed: dismissed }),
      passkeys: [],
      setPasskeys: (passkeys) => set({ passkeys }),
    }),
    {
      name: "moddns-storage",
      // Persist only necessary lightweight UI state
      partialize: (state) => ({
        activeProfile: state.activeProfile,
        verificationBannerDismissed: state.verificationBannerDismissed,
        connectionStatusVisible: state.connectionStatusVisible,
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
      ipv6: '2606:4700:4700::1111', // TODO: replace with real IPv6 when available
      domain: envDomain,
      dohEndpoint: dnsOverHTTPS,
    } as DerivedProfileData;
  }, [activeProfile, envDomain, primaryIp]);
}
