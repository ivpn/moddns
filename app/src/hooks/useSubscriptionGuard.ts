import { useAppStore } from "@/store/general";

export function useSubscriptionGuard() {
  const status = useAppStore(s => s.subscriptionStatus);
  return {
    isLimited: status === 'limited_access',
    isPendingDelete: status === 'pending_delete',
    isRestricted: status === 'limited_access' || status === 'pending_delete',
    canMutate: status === 'active' || status === 'grace_period' || status === null,
  };
}
