import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useSubscriptionGuard } from '@/hooks/useSubscriptionGuard';
import { useAppStore } from '@/store/general';

describe('useSubscriptionGuard', () => {
    beforeEach(() => {
        useAppStore.getState().setSubscriptionStatus(null);
        useAppStore.getState().setSubscriptionDeletionScheduled(false);
    });

    it('active subscription can mutate and is neither restricted nor retired', () => {
        useAppStore.getState().setSubscriptionStatus('active');
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.canMutate).toBe(true);
        expect(result.current.isRestricted).toBe(false);
        expect(result.current.isPendingDelete).toBe(false);
        expect(result.current.isRetired).toBe(false);
    });

    it('expiry-driven pending_delete is pending-delete but NOT retired', () => {
        useAppStore.getState().setSubscriptionStatus('pending_delete');
        useAppStore.getState().setSubscriptionDeletionScheduled(false);
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isPendingDelete).toBe(true);
        expect(result.current.isRetired).toBe(false);
    });

    it('signup-reset retirement is both pending-delete and retired', () => {
        useAppStore.getState().setSubscriptionStatus('pending_delete');
        useAppStore.getState().setSubscriptionDeletionScheduled(true);
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isPendingDelete).toBe(true);
        expect(result.current.isRetired).toBe(true);
    });

    it('deletion_scheduled without pending_delete status does not mark retired', () => {
        // Defensive: isRetired requires the pending_delete status too.
        useAppStore.getState().setSubscriptionStatus('active');
        useAppStore.getState().setSubscriptionDeletionScheduled(true);
        const { result } = renderHook(() => useSubscriptionGuard());
        expect(result.current.isRetired).toBe(false);
    });
});
