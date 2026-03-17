import { useState, useEffect, useRef } from 'react';
import axios from 'axios';
import { customAlphabet } from 'nanoid';
import { useAppStore } from '@/store/general';

export interface DnsCheckResponse {
  status: string;
  profile_id: string;
}

interface StatusInfo {
  badge: { text: string; className: string };
  message: string;
  messageColor: string;
  isCurrentProfile: boolean;
}

export function useDnsConnectionStatus(pollMs: number = 5000, options?: { enabled?: boolean }) {
  const enabled = options?.enabled ?? true;
  const [dnsCheckResponse, setDnsCheckResponse] = useState<DnsCheckResponse>({
    status: '',
    profile_id: '',
  });
  const [isLoading, setIsLoading] = useState(true);
  // Keep the last non-loading, non-error status to prevent UI flicker between polls
  const lastStableStatusRef = useRef<StatusInfo | null>(null);
  const [error, setError] = useState<string>('');
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  const activeProfile = useAppStore((state) => state.activeProfile);
  const profiles = useAppStore((state) => state.profiles);

  const executeDnsCheck = async () => {
    try {
      setError('');
      const alphabet = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
      const nanoid = customAlphabet(alphabet, 12);
      const randID = nanoid();
      const profileId = activeProfile?.profile_id || '';
      const subdomain = `${randID}-${profileId}`;
      const dnsCheckDomain = import.meta.env.VITE_DNS_CHECK_DOMAIN || 'test.moddns.net';
      const url = `https://${subdomain}.${dnsCheckDomain}/`;
      const response = await axios.get(url);
      if (response.status === 200) {
        setDnsCheckResponse(response.data);
        setIsLoading(false);
      }
    } catch (err: unknown) {
      const axiosError = err as { response?: { status?: number; data?: { error?: string } } };
      if (axiosError?.response?.status === 404 && axiosError?.response?.data?.error === 'disconnected') {
        setDnsCheckResponse({
          status: 'disconnected',
          profile_id: '',
        });
        setIsLoading(false);
        setError('');
      } else {
        setError('Failed to check DNS status');
        setIsLoading(false);
        console.error('DNS check error:', err);
      }
    }
  };

  useEffect(() => {
    if (!enabled) {
      // If disabled, clear any existing interval and mark as not loading to avoid spinner hanging
      if (intervalRef.current) clearInterval(intervalRef.current);
      return;
    }
    executeDnsCheck();
    intervalRef.current = setInterval(() => executeDnsCheck(), pollMs);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeProfile?.profile_id, enabled, pollMs]);

  const getCurrentProfileName = () => {
    if (!profiles || !dnsCheckResponse.profile_id) return 'Unknown Profile';
    const profile = profiles.find(p => p.profile_id === dnsCheckResponse.profile_id);
    return profile?.name || 'Unknown Profile';
  };

  const getStatusInfo = (): StatusInfo => {
    // Build fresh status when not in loading state
    let current: StatusInfo | null = null;
    if (!isLoading) {
      if (error) {
        current = { badge: { text: 'Error', className: 'bg-[var(--tailwind-colors-red-600)]' }, message: `Unable to check DNS status.`, messageColor: 'text-[var(--tailwind-colors-red-400)]', isCurrentProfile: false };
      } else if (dnsCheckResponse.status === 'ok') {
        const isCurrentProfile = activeProfile?.profile_id === dnsCheckResponse.profile_id;
        if (isCurrentProfile) {
          current = { badge: { text: 'Connected', className: 'bg-[var(--tailwind-colors-rdns-600)]' }, message: 'This device or browser is currently using modDNS with this profile.', messageColor: 'text-[var(--tailwind-colors-rdns-800)]', isCurrentProfile };
        } else {
          current = { badge: { text: 'Different Profile', className: 'bg-[var(--tailwind-colors-orange-500)]' }, message: `This device or browser is using modDNS with profile: ${getCurrentProfileName()}`, messageColor: 'text-[var(--tailwind-colors-red-400)]', isCurrentProfile };
        }
      } else if (dnsCheckResponse.status === 'disconnected') {
        current = { badge: { text: 'Disconnected', className: 'bg-[var(--tailwind-colors-red-600)]' }, message: 'This device or browser is not using modDNS.', messageColor: 'text-[var(--tailwind-colors-red-400)]', isCurrentProfile: false };
      } else {
        current = { badge: { text: 'Checking...', className: 'bg-[var(--tailwind-colors-slate-800)]' }, message: 'Checking DNS configuration...', messageColor: 'text-[var(--tailwind-colors-slate-100)]', isCurrentProfile: false };
      }
      // Cache this stable status for reuse during transient loading frames
      lastStableStatusRef.current = current;
      return current;
    }

    // Loading state: use previous stable status if available to avoid flicker
    if (lastStableStatusRef.current) {
      return { ...lastStableStatusRef.current, badge: { text: 'Checking...', className: 'bg-[var(--tailwind-colors-slate-800)]' } };
    }
    // Initial load fallback for first render
    return { badge: { text: 'Checking...', className: 'bg-[var(--tailwind-colors-slate-800)]' }, message: 'Checking DNS configuration...', messageColor: 'text-[var(--tailwind-colors-slate-100)]', isCurrentProfile: false };
  };

  return { dnsCheckResponse, isLoading, error, status: getStatusInfo(), refresh: executeDnsCheck, enabled };
}
