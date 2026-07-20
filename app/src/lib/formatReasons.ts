// formatReasons — map raw proxy reason tokens to human-readable chips.
//
// Source of truth: docs/specs/logs-reason-display-behaviour.md
// If the chip mapping changes, update that spec and formatReasons.test.ts with
// matching `tableRef: logs-reason-display-behaviour #N` annotations.

export type ReasonKind = 'blocklist' | 'service' | 'custom_rule' | 'default' | 'subdomain' | 'dnssec';

export interface FormattedReason {
    kind: ReasonKind;
    label: string;
}

const BLOCKLIST_PREFIX = 'blocklist: ';
const SERVICE_PREFIX = 'service: ';

/**
 * Convert stored proxy reason tokens into de-duplicated, ordered display chips.
 *
 * @param reasons        Raw tokens from `ModelQueryLog.reasons` (order not assumed).
 * @param blocklistNames Optional id → display-name map for `blocklist: <id>`.
 * @param serviceNames   Optional id → display-name map for `service: <id>`.
 */
export function formatReasons(
    reasons: string[],
    blocklistNames?: Record<string, string>,
    serviceNames?: Record<string, string>,
): FormattedReason[] {
    if (!reasons || reasons.length === 0) return [];

    // First-seen order preserved; a Set guards against duplicate ids.
    const blocklistIds: string[] = [];
    const blocklistIdSet = new Set<string>();
    const serviceIds: string[] = [];
    const serviceIdSet = new Set<string>();
    let hasGenericBlocklist = false;
    let hasGenericService = false;
    let hasSubdomain = false;
    let hasCustomRule = false;
    let hasDefault = false;
    let hasDnssecFailed = false;

    for (const reason of reasons) {
        if (reason === 'dnssec_failed') {
            hasDnssecFailed = true;
            continue;
        }
        if (reason.startsWith(BLOCKLIST_PREFIX)) {
            const id = reason.slice(BLOCKLIST_PREFIX.length);
            if (!blocklistIdSet.has(id)) {
                blocklistIdSet.add(id);
                blocklistIds.push(id);
            }
        } else if (reason === 'blocklists') {
            hasGenericBlocklist = true;
        } else if (reason === 'blocklists_subdomains_rule') {
            hasSubdomain = true;
        } else if (reason.startsWith(SERVICE_PREFIX)) {
            const id = reason.slice(SERVICE_PREFIX.length);
            if (!serviceIdSet.has(id)) {
                serviceIdSet.add(id);
                serviceIds.push(id);
            }
        } else if (reason === 'services') {
            hasGenericService = true;
        } else if (reason === 'custom_rules') {
            hasCustomRule = true;
        } else if (reason === 'default_rule') {
            hasDefault = true;
        }
        // Unknown tokens are ignored.
    }

    const chips: FormattedReason[] = [];
    const subdomainSuffix = hasSubdomain ? ' (subdomain)' : '';

    // DNSSEC validation failure — shown first; explains an otherwise-opaque SERVFAIL.
    if (hasDnssecFailed) {
        chips.push({ kind: 'dnssec', label: 'DNSSEC validation failed' });
    }

    // Blocklist tier — specific ids collapse the generic token; the subdomain
    // qualifier folds into the chip label rather than becoming its own chip.
    if (blocklistIds.length > 0) {
        for (const id of blocklistIds) {
            const name = blocklistNames?.[id] ?? id;
            chips.push({ kind: 'blocklist', label: `Blocklist: ${name}${subdomainSuffix}` });
        }
    } else if (hasGenericBlocklist || hasSubdomain) {
        // Generic blocklist, or an orphan subdomain rule with nothing to attach to.
        chips.push({ kind: 'blocklist', label: `Blocklist${subdomainSuffix}` });
    }

    // Service tier — specific ids collapse the generic token.
    if (serviceIds.length > 0) {
        for (const id of serviceIds) {
            const name = serviceNames?.[id] ?? id;
            chips.push({ kind: 'service', label: `Service: ${name}` });
        }
    } else if (hasGenericService) {
        chips.push({ kind: 'service', label: 'Service' });
    }

    if (hasCustomRule) {
        chips.push({ kind: 'custom_rule', label: 'Custom rule' });
    }

    if (hasDefault) {
        chips.push({ kind: 'default', label: 'Default rule' });
    }

    return chips;
}

export default formatReasons;
