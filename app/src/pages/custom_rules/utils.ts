export const MAX_RULES_PER_BATCH = 20;

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
