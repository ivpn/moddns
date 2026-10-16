import { AlertTriangle, Info } from "lucide-react";
import { useAppStore } from "@/store/general";
import {
    CUSTOM_RULES_USAGE_NOTICE_AT,
    MAX_CUSTOM_RULES_PER_ACCOUNT,
    countAccountCustomRules,
} from "@/pages/custom_rules/utils";

/**
 * Account-wide custom rules usage, shown on the custom rules page once the
 * account nears the cap. The cap counts every profile, so a profile with few
 * rules can still be refused; this explains why before it happens.
 */
export default function CustomRulesAccountLimitBanner() {
    const activeProfile = useAppStore((state) => state.activeProfile);
    const profiles = useAppStore((state) => state.profiles);
    const used = countAccountCustomRules(profiles, activeProfile);

    if (used < CUSTOM_RULES_USAGE_NOTICE_AT) return null;

    const atCap = used >= MAX_CUSTOM_RULES_PER_ACCOUNT;
    const usage = `${used.toLocaleString()} of ${MAX_CUSTOM_RULES_PER_ACCOUNT.toLocaleString()} custom rules used across all profiles.`;

    return (
        <div
            role={atCap ? "alert" : "status"}
            className={`flex items-center gap-3 w-full rounded-lg border px-4 py-3 ${atCap
                ? "border-yellow-600 dark:border-yellow-400"
                : "border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)]"}`}
        >
            {atCap
                ? <AlertTriangle className="w-5 h-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0" />
                : <Info className="w-5 h-5 text-[var(--tailwind-colors-rdns-500)] flex-shrink-0" />}
            <div className="min-w-0">
                <p className="font-['Figtree',Helvetica] font-semibold text-[var(--tailwind-colors-slate-50)] text-sm leading-5">
                    {atCap ? "Custom rule limit reached" : "Approaching the custom rule limit"}
                </p>
                <p className="font-['Figtree',Helvetica] text-[var(--tailwind-colors-slate-300)] text-sm leading-5 mt-0.5">
                    {usage}
                    {atCap && " New rules won't be accepted until you remove some rules from any profile."}
                </p>
            </div>
        </div>
    );
}
