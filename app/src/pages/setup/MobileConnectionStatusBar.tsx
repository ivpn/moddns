import { useDnsConnectionStatus } from '@/hooks/useDnsConnectionStatus';
import { useSubscriptionGuard } from '@/hooks/useSubscriptionGuard';
import { Badge } from '@/components/ui/badge';
// Removed Alert / Card usage per request; using custom div styling

// Compact mobile-only connection status bar (shown on /setup)
export const MobileConnectionStatusBar: React.FC = () => {
    // PendingDeleteGuard redirects PD users off /setup, but if a transient
    // render slips through we must not fire the connection test — DNS is
    // stopped for PD subs and the result would only ever be 'disconnected'.
    const { isPendingDelete } = useSubscriptionGuard();
    const { status } = useDnsConnectionStatus(7000, { enabled: !isPendingDelete }); // slower poll mobile
    const { badge, message, messageColor } = status;

    if (isPendingDelete) return null;

    return (
        <div data-testid="conn-mobile-root" className="w-full max-w-[630px] rounded-md border border-[var(--shadcn-ui-app-border)] px-3 py-3">
            <div className="flex items-center justify-between w-full gap-3 mb-2">
                <div data-testid="conn-mobile-label" className="font-bold text-[var(--tailwind-colors-slate-50)] text-sm leading-4 whitespace-nowrap font-['Roboto_Mono-Bold',Helvetica]">Status</div>
                <Badge data-testid="conn-mobile-badge" className={`${badge.className} text-[11px] px-2.5 py-1 rounded-sm whitespace-nowrap`}>{badge.text}</Badge>
            </div>
            <div className="flex flex-col gap-1 w-full">
                <div data-testid="conn-mobile-message" className={`text-[12px] leading-5 font-['Roboto_Flex-Regular',Helvetica] ${messageColor} break-words`}>{message}</div>
            </div>
        </div>
    );
};
