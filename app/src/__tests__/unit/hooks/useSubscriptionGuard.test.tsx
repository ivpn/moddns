import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useSubscriptionGuard } from '@/hooks/useSubscriptionGuard';
import { useAppStore } from '@/store/general';

describe('useSubscriptionGuard', () => {
    beforeEach(() => {
        useAppStore.getState().setSubscriptionStatus(null);
        useAppStore.getState().setSubscriptionDeletionScheduled(false);
    });

    it('active subscription can mutate and is neither restricted nor cut off', () => {
        useAppStore.getState().setSubscriptionStatus('active');
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.canMutate).toBe(true);
        expect(result.current.isRestricted).toBe(false);
        expect(result.current.isCutOff).toBe(false);
        expect(result.current.isInactive).toBe(false);
        expect(result.current.isPendingDelete).toBe(false);
        expect(result.current.isRetired).toBe(false);
    });

    it('limited_access is restricted but not cut off', () => {
        useAppStore.getState().setSubscriptionStatus('limited_access');
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isLimited).toBe(true);
        expect(result.current.isRestricted).toBe(true);
        expect(result.current.isCutOff).toBe(false);
    });

    it('inactive is cut off + restricted but NOT retired (expiry/Standard/outage)', () => {
        useAppStore.getState().setSubscriptionStatus('inactive');
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isInactive).toBe(true);
        expect(result.current.isCutOff).toBe(true);
        expect(result.current.isRestricted).toBe(true);
        expect(result.current.isPendingDelete).toBe(false);
        expect(result.current.isRetired).toBe(false);
        expect(result.current.canMutate).toBe(false);
    });

    it('pending_delete (signup-reset) is cut off, pending-delete AND retired', () => {
        useAppStore.getState().setSubscriptionStatus('pending_delete');
        useAppStore.getState().setSubscriptionDeletionScheduled(true);
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isPendingDelete).toBe(true);
        expect(result.current.isRetired).toBe(true);
        expect(result.current.isCutOff).toBe(true);
        expect(result.current.isRestricted).toBe(true);
        expect(result.current.isInactive).toBe(false);
    });

    it('deletion_scheduled without pending_delete status does not mark retired', () => {
        // Defensive: isRetired requires the pending_delete status too.
        useAppStore.getState().setSubscriptionStatus('active');
        useAppStore.getState().setSubscriptionDeletionScheduled(true);
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isRetired).toBe(false);
    });
});
