export const MAX_RULES_PER_BATCH = 20;

// CUSTOM_RULES_EXPORT_LIMIT mirrors the backend's per-profile export cap
// (model.ExportedCustomRulesLimit): a profile export includes only the first
// this-many custom rules (oldest-first). Rules beyond it still apply locally.
export const CUSTOM_RULES_EXPORT_LIMIT = 1000;

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

export type RuleList = "denylist" | "allowlist";

/** The `?list=` value of the Custom rules page; anything else opens the denylist. */
export function parseRuleList(value: string | null | undefined): RuleList {
    return value === "allowlist" ? "allowlist" : "denylist";
}

export const customRulesPath = (list: RuleList): string => `/custom-rules?list=${list}`;
