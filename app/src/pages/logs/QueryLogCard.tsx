import { useId, useState, type JSX } from "react";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { formatDistanceToNow, parseISO, format } from "date-fns";
import { Clock, ShieldPlus } from "lucide-react";

import { Badge } from "@/components/ui/badge"; // still used for Blocked status only
import { Button } from "@/components/ui/button";
import { Tooltip } from "@/components/ui/tooltip";
import { ReasonBadges } from "@/components/ui/ReasonBadges";
import { cn, INTERACTIVE_CARD } from "@/lib/utils";
import type { ModelQueryLog } from "@/api/client";
import type { ConsolidatedLogGroup } from "@/lib/consolidateLogs";

interface QueryLogCardProps {
    log: ModelQueryLog;
    /**
     * Consolidation group this row represents (issue #161). When `count > 1` the row shows a
     * ×N badge and the expanded panel aggregates the members. Omitted / count 1 → single-entry
     * row, rendered identically to before this feature.
     */
    group?: ConsolidatedLogGroup;
    isLast?: boolean;
    lastLogRef?: (node: HTMLDivElement | null) => void;
    onQuickRule?: (domain?: string, defaultAction?: "denylist" | "allowlist") => void;
    quickRuleRestricted?: boolean;
    blocklistNames?: Record<string, string>;
    serviceNames?: Record<string, string>;
    /** Called the first time this row is expanded (used to dismiss the one-time mobile hint). */
    onExpand?: () => void;
}

const QueryLogCard = ({ log, group, isLast, lastLogRef, onQuickRule, quickRuleRestricted, blocklistNames, serviceNames, onExpand }: QueryLogCardProps): JSX.Element | null => {
    // Consolidation: count>1 means this card stands in for a run of adjacent duplicate queries.
    const count = group?.count ?? 1;
    const isConsolidated = count > 1;
    // If domain logging is disabled, dns_request.domain may be absent. Provide a placeholder.
    const rawDomain = log.dns_request?.domain;
    const normalizedDomain = rawDomain ? rawDomain.replace(/\.$/, "") : undefined;
    const displayDomain = normalizedDomain ?? rawDomain;
    const quickRuleAvailable = Boolean(normalizedDomain);
    const isBlocked = log.status === "blocked";
    const isProcessed = log.status === "processed";
    const quickRuleDisabled = !quickRuleAvailable || quickRuleRestricted;
    const quickRuleTooltip = quickRuleRestricted
        ? "Feature unavailable in limited access mode"
        : quickRuleAvailable ? "Create a custom rule" : "Domain unavailable";
    const handleQuickRule = () => {
        if (quickRuleDisabled) return;
        const defaultAction = isBlocked ? "allowlist" : "denylist";
        onQuickRule?.(normalizedDomain, defaultAction);
    };
    const quickRuleButtonClasses = isBlocked
        ? "bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-slate-900)] hover:!text-[var(--tailwind-colors-rdns-600)]"
        : isProcessed
            ? "bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-100)] hover:!bg-[var(--tailwind-colors-red-600)] hover:!text-[var(--tailwind-colors-slate-50)]"
            : "bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-slate-900)] hover:!text-[var(--tailwind-colors-rdns-600)]";
    // Quick-rule is the ONLY control excluded from the whole-card expand overlay; its wrapper
    // sits above the overlay (relative z-20) so it stays clickable.
    const renderQuickRuleButton = (wrapperClassName: string) => (
        <div className={wrapperClassName}>
            <Tooltip content={quickRuleTooltip} side="top" align="center" delay={150}>
                {/* span hosts cursor-not-allowed because the disabled button itself doesn't receive pointer events */}
                <span className={quickRuleRestricted ? 'inline-block cursor-not-allowed' : undefined}>
                    <Button
                        variant="ghost"
                        size="icon"
                        type="button"
                        aria-label="Quick custom rule"
                        onClick={handleQuickRule}
                        disabled={quickRuleDisabled}
                        className={`h-11 w-11 md:h-9 md:w-9 min-h-0 p-0 aspect-square rounded-full disabled:opacity-40 ${quickRuleButtonClasses}`}
                        data-testid="logs-quick-rule-button"
                    >
                        <ShieldPlus className="size-5 md:size-4" />
                    </Button>
                </span>
            </Tooltip>
        </div>
    );

    // Whole-card expand: every row is expandable (blocked and processed, with or without reasons).
    // There is no visible chevron — expandability is signalled by the hover lift (desktop),
    // the press/active feedback (both), and a one-time hint (mobile, owned by the Logs page).
    const reasons = log.reasons ?? [];
    const hasReasons = reasons.length > 0;
    const [expanded, setExpanded] = useState(false);
    const panelId = useId();
    const toggleExpanded = () => setExpanded(v => {
        const next = !v;
        if (next) onExpand?.();
        return next;
    });

    // Device ID: backend allows up to 36 chars; truncate only for mobile (<=768px)
    const { isMobile } = useScreenDetector();
    const rawDeviceId = log.device_id || '';
    let deviceIdOrIp = rawDeviceId;
    if (!rawDeviceId) deviceIdOrIp = log.client_ip || '';
    else if (isMobile) deviceIdOrIp = rawDeviceId.slice(0, 20);
    else deviceIdOrIp = rawDeviceId.slice(0, 36);

    const DOMAIN_TRUNCATE_THRESHOLD = 65; // existing logic threshold
    const isDomainTruncatable = displayDomain ? displayDomain.length > DOMAIN_TRUNCATE_THRESHOLD : false;
    const truncatedDomain = displayDomain && isDomainTruncatable ? displayDomain.slice(0, DOMAIN_TRUNCATE_THRESHOLD) + '…' : displayDomain;
    const protocolLabel = log?.protocol ? log.protocol.toUpperCase() : '—';

    // DNSSEC status shown inline next to the protocol as a plain text badge (styled like the
    // protocol label — no outline/background):
    //   - validated (AD bit true)     -> brand-coloured "DNSSEC"
    //   - failed (bogus/misconfigured) -> red "DNSSEC" (recursor SERVFAILed on validation)
    // Neither shows for domains without a DNSSEC signal.
    const dnssecValidated = log.dns_request?.dnssec === true;
    const dnssecFailed = reasons.includes('dnssec_failed');
    const dnssecShown = dnssecValidated || dnssecFailed;
    // When reserveWhenHidden is set (desktop), the badge is always rendered — invisible
    // when there's no DNSSEC — so it reserves a constant slot and the protocol label
    // never shifts depending on whether DNSSEC is shown. The testid/color are only
    // applied when actually shown.
    const renderDnssecBadge = (className?: string, reserveWhenHidden = false) => {
        if (!dnssecShown && !reserveWhenHidden) return null;
        return (
            <span
                data-testid={dnssecShown ? 'querylog-dnssec-badge' : undefined}
                data-dnssec={dnssecShown ? (dnssecFailed ? 'failed' : 'validated') : undefined}
                aria-hidden={!dnssecShown || undefined}
                className={cn(
                    // Match the protocol label typography exactly (font/size/weight/leading).
                    "font-text-xs-leading-4-semibold font-semibold text-[10px] md:text-[length:var(--text-xs-leading-4-semibold-font-size)] tracking-wide leading-4 md:leading-[var(--text-xs-leading-4-semibold-line-height)] uppercase whitespace-nowrap",
                    dnssecShown
                        ? (dnssecFailed ? "text-[var(--tailwind-colors-red-600)]" : "text-[var(--tailwind-colors-rdns-600)]")
                        // Reserve placeholder (desktop only): take no vertical line in the tablet
                        // stack (md), but keep reserving horizontal space in the lg row.
                        : "opacity-0 pointer-events-none select-none md:hidden lg:inline-block",
                    className,
                )}
            >
                DNSSEC
            </span>
        );
    };

    // Detail-grid field: uppercase micro-label + selectable value (optionally coloured).
    const renderDetailField = (label: string, value: string, testid: string, valueClassName?: string) => (
        <div className="min-w-0">
            <dt className="text-[10px] uppercase tracking-wide font-semibold text-[var(--tailwind-colors-slate-100)]">{label}</dt>
            <dd className={cn("text-xs break-all select-text text-[var(--tailwind-colors-slate-50)]", valueClassName)} data-testid={testid}>{value}</dd>
        </div>
    );

    // DNSSEC has three distinct states — keep them clearly worded and colour-coded:
    //   failed (bogus)   -> "Validation failed" (red)     — signatures broken
    //   validated (AD=1) -> "Validated" (brand/green)     — authentic
    //   unsigned         -> "No DNSSEC" (muted)           — domain isn't signed
    const dnssecDetail = dnssecFailed
        ? { text: 'Validation failed', className: 'text-[var(--tailwind-colors-red-600)]' }
        : dnssecValidated
            ? { text: 'Validated', className: 'text-[var(--tailwind-colors-rdns-600)]' }
            : { text: 'No DNSSEC', className: 'text-[var(--tailwind-colors-slate-200)]' };

    // Consolidation badge: a small non-interactive "×N" pill shown next to the domain when
    // this card merges multiple adjacent duplicate queries. Styled like the protocol/DNSSEC
    // micro-labels. Plain text, so it can sit under the whole-card toggle overlay.
    const renderCountBadge = (className?: string) => {
        if (!isConsolidated) return null;
        return (
            <span
                data-testid="querylog-count-badge"
                data-count={count}
                aria-label={`${count} duplicate queries`}
                className={cn(
                    "shrink-0 font-semibold text-[10px] tracking-wide uppercase whitespace-nowrap text-[var(--tailwind-colors-rdns-600)]",
                    className,
                )}
            >
                ×{count}
            </span>
        );
    };

    // Expanded-panel field values: aggregate across members when consolidated, else fall back
    // to the representative's single value.
    const queryTypeText = isConsolidated ? group?.queryTypes.join(', ') : log.dns_request?.query_type;
    const responseCodeText = isConsolidated ? group?.responseCodes.join(', ') : log.dns_request?.response_code;
    // A group only shows a first–last time RANGE when its endpoints differ at second granularity.
    // A + AAAA fired back-to-back land in the same second, so those collapse to a single "Time"
    // (like a non-consolidated row); groups that genuinely span >=1s (e.g. grouped blocked queries)
    // keep the range.
    const secKey = (ts?: string) => (ts ? format(parseISO(ts), "yyyy-MM-dd'T'HH:mm:ss") : undefined);
    const hasTimeRange = Boolean(
        isConsolidated && group?.firstTimestamp && group?.lastTimestamp &&
        secKey(group.firstTimestamp) !== secKey(group.lastTimestamp)
    );
    const timeText = hasTimeRange
        ? `${format(parseISO(group!.firstTimestamp!), "MMMM d, yyyy 'at' hh:mm:ss a")} – ${format(parseISO(group!.lastTimestamp!), "hh:mm:ss a")}`
        : (log.timestamp ? format(parseISO(log.timestamp), "MMMM d, yyyy 'at' hh:mm:ss a") : "—");

    return (
        <div
            ref={isLast ? lastLogRef : undefined}
            className={cn(
                "w-full bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-[var(--primitives-radius-radius-md)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent",
                INTERACTIVE_CARD,
                "relative hover:z-10 hover:shadow-lg hover:border-[var(--tailwind-colors-rdns-600)]",
                // Press/active feedback (works on touch where there is no hover) — subtle tint on tap.
                "active:bg-[var(--shadcn-ui-app-accent)] dark:active:bg-[var(--shadcn-ui-app-accent)]"
            )}
        >
            {/* Whole-card expand/collapse trigger: a real button (native keyboard/focus/aria).
                Spans the ENTIRE card (absolute inset-0 on the card root), so clicking anywhere —
                the header row OR the expanded detail panel — toggles it. Collapsed, the panel is
                0-height so the button only covers the header. Quick-rule (z-20) stays above it. */}
            <button
                type="button"
                onClick={toggleExpanded}
                aria-expanded={expanded}
                aria-controls={panelId}
                aria-label={expanded ? "Hide query details" : "Show query details"}
                data-testid="querylog-card-toggle"
                className="absolute inset-0 z-10 w-full cursor-pointer rounded-[var(--primitives-radius-radius-md)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--tailwind-colors-rdns-600)]"
            />
            <div className="relative flex h-auto md:h-[66px] items-stretch md:items-center justify-between gap-3 md:gap-4 px-3 md:pt-[var(--tailwind-primitives-gap-gap-3)] md:pr-[var(--tailwind-primitives-gap-gap-4)] md:pb-[var(--tailwind-primitives-gap-gap-3)] md:pl-[var(--tailwind-primitives-gap-gap-4)] min-w-0 py-2 md:py-0 min-h-[64px]">
                <div className="flex items-center gap-3 relative min-w-0 flex-1">
                    <div className="flex flex-col gap-1 w-full">
                        <div className="flex items-start gap-2">
                            <div className="inline-flex items-center gap-2 relative min-w-0 flex-1">
                                <div className="relative flex flex-col gap-1 min-w-0">
                                    <div className="hidden md:flex items-center gap-2 font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-foreground text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] truncate max-w-[200px] md:max-w-[560px] lg:max-w-[560px]">
                                        {displayDomain ? (
                                            <span
                                                className="truncate"
                                                data-testid={!isDomainTruncatable ? 'querylog-domain-full' : 'querylog-domain-truncated-desktop'}
                                            >
                                                {isDomainTruncatable ? truncatedDomain : displayDomain}
                                            </span>
                                        ) : (
                                            '-'
                                        )}
                                        {renderCountBadge()}
                                    </div>
                                </div>
                            </div>
                        </div>
                        {isMobile && (
                            <div className="md:hidden flex flex-col gap-2">
                                <div className="flex items-center justify-between gap-3">
                                    <div className="flex items-center gap-3 text-[10px] uppercase font-semibold tracking-wide text-[var(--tailwind-colors-rdns-600)]">
                                        <span>{protocolLabel}</span>
                                        {dnssecShown && renderDnssecBadge()}
                                        {isBlocked && (
                                            <Badge
                                                className="inline-flex items-center justify-center px-2 py-0.5 bg-[var(--tailwind-colors-red-600)] rounded border-0 h-5"
                                            >
                                                <span className="font-text-xs-leading-4-semibold text-[10px] leading-4 text-white font-semibold">Blocked</span>
                                            </Badge>
                                        )}
                                    </div>
                                    <div className="flex items-center gap-3">
                                        <div className="flex items-center gap-1 max-w-[200px] font-text-sm-leading-5-semibold text-[var(--tailwind-colors-slate-50)] text-xs text-right">
                                            <span data-testid="querylog-device-id-full">{deviceIdOrIp}</span>
                                        </div>
                                        {renderQuickRuleButton("flex-shrink-0 relative z-20")}
                                    </div>
                                </div>
                                <div className="flex items-center gap-x-2 gap-y-2 min-w-0 flex-wrap">
                                    <div className="flex items-center gap-2 min-w-0 flex-1">
                                        <div className="relative flex flex-1 items-center gap-2 font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-foreground text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] truncate max-w-full text-left min-w-0">
                                            {displayDomain ? (
                                                <span
                                                    className="truncate"
                                                    data-testid={isDomainTruncatable ? 'querylog-domain-truncated' : 'querylog-domain-full'}
                                                >
                                                    {isDomainTruncatable ? truncatedDomain : displayDomain}
                                                </span>
                                            ) : (
                                                '-'
                                            )}
                                            {renderCountBadge()}
                                        </div>
                                    </div>
                                    <div className="flex-shrink-0 ml-auto">
                                        <TimestampDisplay timestamp={log.timestamp} />
                                    </div>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
                {!isMobile && (
                    <div className="flex items-stretch md:items-center gap-3.5 md:gap-3 relative flex-[0_0_auto] min-w-0">
                        <div className="hidden md:flex flex-col lg:flex-row items-end lg:items-center gap-1 lg:gap-2.5 flex-shrink-0">
                            <div className="relative w-[60px] md:w-auto lg:w-[100px] font-text-xs-leading-4-semibold font-semibold text-[10px] md:text-[length:var(--text-xs-leading-4-semibold-font-size)] text-[var(--tailwind-colors-rdns-600)] text-left md:text-right lg:text-center tracking-wide leading-4 md:leading-[var(--text-xs-leading-4-semibold-line-height)] uppercase order-1">
                                {protocolLabel}
                            </div>
                            {renderDnssecBadge("order-2", true)}
                            <Badge className={`order-3 lg:order-0 inline-flex items-center justify-center px-2 py-0.5 md:pt-[var(--tailwind-primitives-padding-p-0-5)] md:pr-[var(--tailwind-primitives-padding-p-2-5)] md:pb-[var(--tailwind-primitives-padding-p-0-5)] md:pl-[var(--tailwind-primitives-padding-p-2-5)] bg-[var(--tailwind-colors-red-600)] rounded border-0 h-5 md:h-auto ${!isBlocked ? 'opacity-0 pointer-events-none select-none md:hidden lg:inline-flex' : ''}`} aria-hidden={!isBlocked}>
                                <span className="font-text-xs-leading-4-semibold text-[10px] md:text-[length:var(--text-xs-leading-4-semibold-font-size)] leading-4 text-white font-semibold">Blocked</span>
                            </Badge>
                        </div>
                        <div className="flex flex-col w-[140px] md:w-[220px] lg:w-[280px] items-end justify-center gap-0.5 md:gap-[var(--tailwind-primitives-gap-gap-0-5)] relative min-w-0 flex-shrink-0">
                            <div className="relative w-full mt-[-1.00px] font-text-sm-leading-5-semibold text-[var(--tailwind-colors-slate-50)] text-xs md:text-[length:var(--text-sm-leading-5-semibold-font-size)] leading-4 md:leading-[var(--text-sm-leading-5-semibold-line-height)] text-right">
                                <span
                                    className="inline-flex items-center gap-1 max-w-full"
                                    data-testid="querylog-device-id-full"
                                >
                                    {deviceIdOrIp}
                                </span>
                            </div>
                            <TimestampDisplay timestamp={log.timestamp} />
                        </div>
                        {renderQuickRuleButton("flex items-center justify-center relative z-20")}
                    </div>
                )}
            </div>
            <div
                id={panelId}
                role="region"
                aria-label="Query details"
                aria-hidden={!expanded}
                data-testid="querylog-expanded-panel"
                data-expanded={expanded}
                className={`grid transition-[grid-template-rows] duration-200 ease-out motion-reduce:transition-none ${expanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
            >
                <div className="overflow-hidden">
                    <div className="border-t border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent px-4 py-3 flex flex-col gap-3 min-w-0">
                        <dl data-testid="querylog-detail-grid" className="grid grid-cols-2 md:grid-cols-3 gap-x-4 gap-y-3 min-w-0">
                            {normalizedDomain !== undefined
                                ? renderDetailField("Domain", normalizedDomain, "querylog-detail-domain")
                                : (
                                    <div className="min-w-0">
                                        <dt className="text-[10px] uppercase tracking-wide font-semibold text-[var(--tailwind-colors-slate-100)]">Domain</dt>
                                        <dd className="text-xs italic select-text text-[var(--tailwind-colors-slate-200)]" data-testid="querylog-detail-domain">Domain logging disabled</dd>
                                    </div>
                                )}
                            {queryTypeText && renderDetailField(isConsolidated ? "Query types" : "Query type", queryTypeText, "querylog-detail-query-type")}
                            {responseCodeText && renderDetailField(isConsolidated ? "Response codes" : "Response code", responseCodeText, "querylog-detail-response-code")}
                            {(log.dns_request?.dnssec !== undefined || dnssecFailed) && renderDetailField("DNSSEC", dnssecDetail.text, "querylog-detail-dnssec", dnssecDetail.className)}
                            {renderDetailField("Protocol", protocolLabel, "querylog-detail-protocol")}
                            {isConsolidated && renderDetailField("Occurrences", String(count), "querylog-detail-occurrences")}
                            {log.client_ip && renderDetailField("Client IP", log.client_ip, "querylog-detail-client-ip")}
                            {log.device_id && renderDetailField("Device ID", log.device_id, "querylog-detail-device-id")}
                            {renderDetailField(hasTimeRange ? "Time range" : "Time", timeText, "querylog-detail-timestamp")}
                        </dl>
                        {hasReasons && (
                            <div className="flex flex-col gap-1.5 min-w-0" data-testid="querylog-reasons">
                                <span className="text-[10px] uppercase tracking-wide font-semibold text-[var(--tailwind-colors-slate-100)]">{(isBlocked || dnssecFailed) ? "Block reason" : "Allow reason"}</span>
                                <ReasonBadges reasons={reasons} blocklistNames={blocklistNames} serviceNames={serviceNames} />
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
};

interface TimestampDisplayProps { timestamp?: string }

// Static relative-time label (Clock icon + "x ago"). The absolute timestamp moves to the panel.
const TimestampDisplay = ({ timestamp }: TimestampDisplayProps) => {
    if (!timestamp) return null;
    const relative = formatDistanceToNow(parseISO(timestamp), { addSuffix: true });
    return (
        <span
            className="relative w-fit font-text-xs-leading-5-normal font-[number:var(--text-xs-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-100)] text-[length:var(--text-xs-leading-5-normal-font-size)] tracking-[var(--text-xs-leading-5-normal-letter-spacing)] leading-[var(--text-xs-leading-5-normal-line-height)] whitespace-nowrap [font-style:var(--text-xs-leading-5-normal-font-style)] inline-flex items-center gap-1 select-text"
        >
            <Clock className="w-3 h-3 opacity-60" />
            <span className="whitespace-nowrap">{relative}</span>
        </span>
    );
};

export default QueryLogCard;
