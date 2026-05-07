import { Info } from "lucide-react";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";

const RESYNC_URL = import.meta.env.VITE_RESYNC_URL || "https://www.ivpn.net/en/account/";

export default function LimitedAccessBanner() {
    const { isLimited } = useSubscriptionGuard();

    if (!isLimited) return null;

    return (
        <div className="flex items-center gap-3 rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />
            <div className="min-w-0">
                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                    Limited Access Mode
                </p>
                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                    Your modDNS account is in limited access mode. To regain full access add time to your{" "}
                    <a href={RESYNC_URL} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">IVPN account</a>.
                </p>
            </div>
        </div>
    );
}
