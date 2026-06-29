import { Info } from "lucide-react";
import { useAppStore } from "@/store/general";
import { CUSTOM_RULES_EXPORT_LIMIT } from "@/pages/custom_rules/utils";

/**
 * Informative banner shown on the custom rules page when the active profile holds
 * more custom rules than the per-profile export cap. All rules still apply here;
 * only a profile *export* is truncated (oldest-first), so the user should know an
 * exported / re-imported copy of this profile won't include the overflow.
 */
export default function CustomRulesExportLimitBanner() {
    const activeProfile = useAppStore((state) => state.activeProfile);
    const ruleCount = activeProfile?.settings?.custom_rules?.length ?? 0;

    if (ruleCount <= CUSTOM_RULES_EXPORT_LIMIT) return null;

    return (
        <div className="flex items-center gap-3 w-full rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] px-4 py-3">
            <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />
            <div className="min-w-0">
                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                    Custom rule export limit
                </p>
                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                    This profile has {ruleCount.toLocaleString()} custom rules — all of them are active here. A
                    profile export includes only the first {CUSTOM_RULES_EXPORT_LIMIT.toLocaleString()} rules
                    per profile (oldest first), so an exported or re-imported copy won't include the rest.
                </p>
            </div>
        </div>
    );
}
