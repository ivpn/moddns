import type { ModelProfile } from "@/api/client/api";

export const MAX_RULES_PER_BATCH = 20;

// Mirrors the backend's model.MaxCustomRulesPerAccount: the total across all of
// an account's profiles that create and import accept.
export const MAX_CUSTOM_RULES_PER_ACCOUNT = 10000;
export const CUSTOM_RULES_USAGE_NOTICE_AT = 9000;

/**
 * Total custom rules across the account. The stored profile list can lag behind
 * rule edits, so the active profile's live rules replace its stored entry.
 */
export function countAccountCustomRules(profiles: ModelProfile[], activeProfile: ModelProfile | null): number {
    const activeId = activeProfile?.profile_id;
    let total = activeProfile?.settings?.custom_rules?.length ?? 0;
    for (const profile of profiles) {
        if (activeId && profile.profile_id === activeId) continue;
        total += profile.settings?.custom_rules?.length ?? 0;
    }
    return total;
}

/**
 * Splits a raw user input string into potential rule values using whitespace and common separators.
 */
export function splitRulesFromInput(raw: string): string[] {
    return raw
        .split(/[\n\r\t,;]+|\s{1,}/g)
        .map((value) => value.trim())
        .filter(Boolean);
}

/**
 * Normalizes a rule value by trimming whitespace and removing a trailing dot, if present.
 */
export function normalizeRuleValue(value: string): string | null {
    const trimmed = value.trim();
    if (!trimmed) {
        return null;
    }
    if (trimmed.endsWith(".")) {
        return trimmed.slice(0, -1);
    }
    return trimmed;
}
