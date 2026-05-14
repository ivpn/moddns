import { Info } from "lucide-react";
import { useAppStore } from "@/store/general";

const RESYNC_URL = import.meta.env.VITE_RESYNC_URL || "https://www.ivpn.net/en/account/";
const RESYNC_LINK = `${RESYNC_URL}?action=sync&service=dns`;

// BetaEndingBanner is rendered alongside LimitedAccessBanner on the same set of
// pages. It targets pre-0.1.8 accounts whose subscription document still carries
// the legacy `type: "Managed"` value -- a one-time migration prompt that
// disappears after the user resyncs (the backend clears `type` to "").
export default function BetaEndingBanner() {
    const subscriptionType = useAppStore(s => s.subscriptionType);

    if (subscriptionType !== "Managed") return null;

    return (
        <div className="flex items-center gap-3 w-full rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />
            <div className="min-w-0">
                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                    modDNS beta ends May 19
                </p>
                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                    To keep access, follow{" "}
                    <a href={RESYNC_LINK} target="_blank" rel="noreferrer" className="!underline !text-[var(--tailwind-colors-slate-300)]">this link</a>
                    {" "}and sync with your IVPN account.
                </p>
            </div>
        </div>
    );
}
