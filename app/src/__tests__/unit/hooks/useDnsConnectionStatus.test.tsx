import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDnsConnectionStatus } from '@/hooks/useDnsConnectionStatus';
import { useAppStore } from '@/store/general';
import axios from 'axios';

vi.mock('axios');

// Helper to advance timers and flush pending promises
async function advance(ms: number) {
    vi.advanceTimersByTime(ms);
    // wait microtasks
    await Promise.resolve();
}

describe('useDnsConnectionStatus', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        vi.clearAllMocks();
        // reset store
        const { setActiveProfile, setProfiles } = useAppStore.getState();
        setProfiles([] as any);
        setActiveProfile(null as any);
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('initially shows checking placeholder message', async () => {
        // prevent immediate resolution causing state update outside act
        (axios.get as any).mockImplementationOnce(() => new Promise(() => { }));
        let resultRef: any;
        await act(async () => {
            const { result } = renderHook(() => useDnsConnectionStatus(5000));
            resultRef = result;
            // allow microtask queue flush
            await Promise.resolve();
        });
        const status = resultRef.current.status;
        expect(status.badge.text).toBe('Checking...');
        expect(status.message).toBe('Checking DNS configuration...');
    });

    it('handles successful ok response with current active profile', async () => {
        const profile = { profile_id: 'p1', id: 'p1', name: 'Profile One' } as any;
        const { setActiveProfile, setProfiles } = useAppStore.getState();
        setProfiles([profile]);
        setActiveProfile(profile);

        (axios.get as any).mockResolvedValueOnce({ status: 200, data: { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' } });

        const { result } = renderHook(() => useDnsConnectionStatus(5000));

        await act(async () => {
            await advance(0); // allow first request promise to resolve
        });

        const status = result.current.status;
        expect(status.badge.text).toBe('Connected');
        expect(status.message).toContain('currently using modDNS with this profile');
    });

    it('handles successful ok response with different profile', async () => {
        const active = { profile_id: 'p1', id: 'p1', name: 'Profile One' } as any;
        const other = { profile_id: 'p2', id: 'p2', name: 'Other Profile' } as any;
        const { setActiveProfile, setProfiles } = useAppStore.getState();
        setProfiles([active, other]);
        setActiveProfile(active);

        (axios.get as any).mockResolvedValueOnce({ status: 200, data: { status: 'ok', profile_id: 'p2', asn: '', asn_organization: 'Org', ip: '1.1.1.1' } });

        const { result } = renderHook(() => useDnsConnectionStatus(5000));

        await act(async () => { await advance(0); });

        const status = result.current.status;
        expect(status.badge.text).toBe('Different Profile');
        expect(status.message).toContain('Other Profile');
    });

    it('handles disconnected 404 case', async () => {
        (axios.get as any).mockRejectedValueOnce({ response: { status: 404, data: { error: 'disconnected' } } });

        const { result } = renderHook(() => useDnsConnectionStatus(5000));

        await act(async () => {
            await advance(0);
        });

        const status = result.current.status;
        expect(status.badge.text).toBe('Disconnected');
        expect(status.message).toBe('This device or browser is not using modDNS.');
    });

    it('handles generic error and shows fallback message', async () => {
        (axios.get as any).mockResolvedValueOnce({ status: 200, data: { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' } });
        // second poll triggers error
        (axios.get as any).mockRejectedValueOnce(new Error('network'));

        const profile = { profile_id: 'p1', id: 'p1', name: 'Profile One' } as any;
        const { setActiveProfile, setProfiles } = useAppStore.getState();
        setProfiles([profile]);
        setActiveProfile(profile);

        const { result } = renderHook(() => useDnsConnectionStatus(10));

        await act(async () => {
            await advance(0); // first success
        });

        await act(async () => {
            await advance(15); // second poll triggers error
        });

        const status = result.current.status;
        expect(status.badge.text).toBe('Error');
        expect(status.message).toBe('Unable to check DNS status.');
    });

    it('keeps previous message during loading between polls', async () => {
        // first success
        (axios.get as any).mockResolvedValueOnce({ status: 200, data: { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' } });
        // second pending: we'll not resolve yet
        let secondResolve: any;
        (axios.get as any).mockImplementationOnce(() => new Promise(res => { secondResolve = res; }));

        const profile = { profile_id: 'p1', id: 'p1', name: 'Profile One' } as any;
        const { setActiveProfile, setProfiles } = useAppStore.getState();
        setProfiles([profile]);
        setActiveProfile(profile);

        const { result } = renderHook(() => useDnsConnectionStatus(10));

        await act(async () => { await advance(0); }); // first resolves

        const stableMessage = result.current.status.message;
        expect(stableMessage.length).toBeGreaterThan(0);

        // advance to trigger second poll (pending)
        await act(async () => { await advance(12); });

        // still shows stable message while loading
        expect(result.current.status.message).toBe(stableMessage);

        // complete second poll with different profile id to simulate change
        secondResolve({ status: 200, data: { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org2', ip: '2.2.2.2' } });
        await act(async () => { await advance(0); });

        expect(result.current.status.message).toContain('modDNS with this profile');
    });

    it('does not send requests when disabled', () => {
        const spy = axios.get as any;
        renderHook(() => useDnsConnectionStatus(50, { enabled: false }));
        // Advance time well beyond one interval
        vi.advanceTimersByTime(500);
        expect(spy).not.toHaveBeenCalled();
    });
});
