import { useAppStore } from "@/store/general";

export function useSubscriptionGuard() {
  const status = useAppStore(s => s.subscriptionStatus);
  const deletionScheduled = useAppStore(s => s.subscriptionDeletionScheduled);
  // "Cut off" = terminal states that stop DNS, restrict the API, and redirect to
  // /account-preferences: `inactive` (expiry / Standard / long outage — NOT
  // deleted, recoverable via resync) and `pending_delete` (signup-reset retired,
  // hard-deleted in 48h).
  const isInactive = status === 'inactive';
  const isPendingDelete = status === 'pending_delete';
  const isCutOff = isInactive || isPendingDelete;
  return {
    isLimited: status === 'limited_access',
    isInactive,
    isPendingDelete,
    // Retired by the signup-reset flow (replaced by a new signup; not
    // recoverable). pending_delete is retired-only.
    isRetired: isPendingDelete && deletionScheduled,
    isCutOff,
    isRestricted: status === 'limited_access' || isCutOff,
    canMutate: status === 'active' || status === 'grace_period' || status === null,
  };
}
