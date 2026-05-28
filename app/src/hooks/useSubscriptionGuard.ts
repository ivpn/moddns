import { useAppStore } from "@/store/general";

export function useSubscriptionGuard() {
  const status = useAppStore(s => s.subscriptionStatus);
  const deletionScheduled = useAppStore(s => s.subscriptionDeletionScheduled);
  return {
    isLimited: status === 'limited_access',
    isPendingDelete: status === 'pending_delete',
    // Retired by the signup-reset flow: a pending_delete account that was
    // replaced by a new signup (not recoverable by adding IVPN time).
    isRetired: status === 'pending_delete' && deletionScheduled,
    isRestricted: status === 'limited_access' || status === 'pending_delete',
    canMutate: status === 'active' || status === 'grace_period' || status === null,
  };
}
