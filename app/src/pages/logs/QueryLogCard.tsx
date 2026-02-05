import { useState, type JSX } from "react";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { formatDistanceToNow, parseISO, format } from "date-fns";
import { Clock, ShieldPlus } from "lucide-react";

import { Badge } from "@/components/ui/badge"; // still used for Blocked status only
import { Button } from "@/components/ui/button";
import { Tooltip } from "@/components/ui/tooltip";
import type { ModelQueryLog } from "@/api/client";

interface QueryLogCardProps {
    log: ModelQueryLog;
    isLast?: boolean;
    lastLogRef?: (node: HTMLDivElement | null) => void;
    onQuickRule?: (domain?: string, defaultAction?: "denylist" | "allowlist") => void;
}

const QueryLogCard = ({ log, isLast, lastLogRef, onQuickRule }: QueryLogCardProps): JSX.Element | null => {
    // If domain logging is disabled, dns_request.domain may be absent. Provide a placeholder.
    const rawDomain = log.dns_request?.domain;
    const normalizedDomain = rawDomain ? rawDomain.replace(/\.$/, "") : undefined;
    const displayDomain = normalizedDomain ?? rawDomain;
    const quickRuleAvailable = Boolean(normalizedDomain);
    const isBlocked = log.status === "blocked";
    const isProcessed = log.status === "processed";
    const quickRuleTooltip = quickRuleAvailable ? "Create a custom rule" : "Domain unavailable";
    const handleQuickRule = () => {
        if (!quickRuleAvailable) return;
        const defaultAction = isBlocked ? "allowlist" : "denylist";
        onQuickRule?.(normalizedDomain, defaultAction);
    };
    const quickRuleButtonClasses = isBlocked
        ? "bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-slate-900)] hover:!text-[var(--tailwind-colors-rdns-600)]"
        : isProcessed
            ? "bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-100)] hover:!bg-[var(--tailwind-colors-red-600)] hover:!text-[var(--tailwind-colors-slate-50)]"
            : "bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:!bg-[var(--tailwind-colors-slate-900)] hover:!text-[var(--tailwind-colors-rdns-600)]";
    const renderQuickRuleButton = (wrapperClassName: string) => (
        <div className={wrapperClassName}>
            <Tooltip content={quickRuleTooltip} side="top" align="center" delay={150}>
                <span>
                    <Button
                        variant="ghost"
                        size="icon"
                        type="button"
                        aria-label="Quick custom rule"
                        onClick={handleQuickRule}
                        disabled={!quickRuleAvailable}
                        className={`h-9 w-9 lg:min-h-0 p-0 aspect-square rounded-full disabled:opacity-40 ${quickRuleButtonClasses}`}
                        data-testid="logs-quick-rule-button"
                    >
                        <ShieldPlus className="w-4 h-4" />
                    </Button>
                </span>
            </Tooltip>
        </div>
    );

    // Track timestamp expansion to increase card height smoothly on mobile
    const [timestampExpanded, setTimestampExpanded] = useState(false);

    // Expansion state for mobile tap-to-expand of truncated domain (device id no longer truncates)
    const [showFullDomainMobile, setShowFullDomainMobile] = useState(false);

    // Device ID: backend allows up to 36 chars; truncate only for mobile (<=768px)
    const { isMobile } = useScreenDetector();
    const rawDeviceId = log.device_id || '';
    let deviceIdOrIp = rawDeviceId;
    if (!rawDeviceId) deviceIdOrIp = log.client_ip || '';
    else if (isMobile) deviceIdOrIp = rawDeviceId.slice(0, 20);
    else deviceIdOrIp = rawDeviceId.slice(0, 36);

    const DOMAIN_TRUNCATE_THRESHOLD = 65; // existing logic threshold
    const MOBILE_EXPANDED_DOMAIN_LIMIT = 50;
    const TIMESTAMP_COLLAPSED_MAX_HEIGHT = 24;
    const TIMESTAMP_EXPANDED_MAX_HEIGHT = 48;
    const isDomainTruncatable = displayDomain ? displayDomain.length > DOMAIN_TRUNCATE_THRESHOLD : false;
    const truncatedDomain = displayDomain && isDomainTruncatable ? displayDomain.slice(0, DOMAIN_TRUNCATE_THRESHOLD) + '…' : displayDomain;
    const mobileExpandedDomain = displayDomain
        ? displayDomain.length > MOBILE_EXPANDED_DOMAIN_LIMIT
            ? displayDomain.slice(0, MOBILE_EXPANDED_DOMAIN_LIMIT) + '…'
            : displayDomain
        : undefined;
    const protocolLabel = log?.protocol ? log.protocol.toUpperCase() : '—';

    return (
        <div
            ref={isLast ? lastLogRef : undefined}
            className="w-full bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-[var(--primitives-radius-radius-md)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent"
        >
            <div className={`flex h-auto md:h-[66px] items-stretch md:items-center justify-between gap-3 md:gap-4 px-3 md:pt-[var(--tailwind-primitives-gap-gap-3)] md:pr-[var(--tailwind-primitives-gap-gap-4)] md:pb-[var(--tailwind-primitives-gap-gap-3)] md:pl-[var(--tailwind-primitives-gap-gap-4)] min-w-0 py-2 md:py-0 transition-all duration-200 ease-out min-h-[64px] ${timestampExpanded ? 'pb-3.5 min-h-[84px]' : ''}`}>
                <div className="flex items-center gap-3 relative min-w-0 flex-1">
                    <div className="flex flex-col gap-1 w-full">
                        <div className="flex items-start gap-2">
                            <div className="inline-flex items-center gap-2 relative min-w-0 flex-1">
                                <div className="relative flex flex-col gap-1 min-w-0">
                                    <div className="hidden md:flex items-center gap-2 font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-foreground text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] truncate max-w-[200px] md:max-w-[480px] lg:max-w-[560px]">
                                        {displayDomain ? (
                                            <Tooltip content={displayDomain} side="top" align="start" delay={150}>
                                                <span
                                                    className="truncate"
                                                    data-testid={!isDomainTruncatable ? 'querylog-domain-full' : 'querylog-domain-truncated-desktop'}
                                                >
                                                    {isDomainTruncatable ? truncatedDomain : displayDomain}
                                                </span>
                                            </Tooltip>
                                        ) : (
                                            '-'
                                        )}
                                    </div>
                                </div>
                            </div>
                        </div>
                        {isMobile && (
                            <div className="md:hidden flex flex-col gap-2">
                                <div className="flex items-center justify-between gap-3">
                                    <div className="flex items-center gap-3 text-[10px] uppercase font-semibold tracking-wide text-[var(--tailwind-colors-rdns-600)]">
                                        <span>{protocolLabel}</span>
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
                                        {renderQuickRuleButton("flex-shrink-0")}
                                    </div>
                                </div>
                                <div className={`flex gap-x-2 gap-y-2 min-w-0 flex-wrap transition-all duration-300 ease-out ${timestampExpanded ? 'items-start' : 'items-center'}`}>
                                    <div className="flex items-center gap-2 min-w-0 flex-1 order-1 transition-all duration-300 ease-out">
                                        <div className="relative flex flex-1 items-center gap-2 font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-foreground text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] truncate max-w-full text-left min-w-0">
                                            {displayDomain ? (
                                                timestampExpanded ? (
                                                    <span
                                                        className="truncate whitespace-nowrap"
                                                        data-testid="querylog-domain-expanded"
                                                    >
                                                        {mobileExpandedDomain}
                                                    </span>
                                                ) : isDomainTruncatable ? (
                                                    <button
                                                        type="button"
                                                        aria-label={showFullDomainMobile ? 'Hide full domain' : 'Show full domain'}
                                                        onClick={() => setShowFullDomainMobile(v => !v)}
                                                        className="truncate focus:outline-none active:scale-[0.98] transition-transform text-left"
                                                        data-testid={showFullDomainMobile ? 'querylog-domain-full' : 'querylog-domain-truncated'}
                                                    >
                                                        {showFullDomainMobile ? displayDomain : truncatedDomain}
                                                    </button>
                                                ) : (
                                                    <span className="truncate" data-testid="querylog-domain-full">{displayDomain}</span>
                                                )
                                            ) : (
                                                '-'
                                            )}
                                        </div>
                                    </div>
                                    <div
                                        className={`${timestampExpanded ? 'order-2 basis-full w-full flex justify-end' : 'order-2 flex-shrink-0 ml-auto self-stretch'} transition-[flex-basis,padding,margin] duration-300 ease-out`}
                                    >
                                        <div
                                            className="overflow-hidden transition-[max-height] duration-300 ease-out"
                                            style={{ maxHeight: timestampExpanded ? TIMESTAMP_EXPANDED_MAX_HEIGHT : TIMESTAMP_COLLAPSED_MAX_HEIGHT }}
                                        >
                                            <TimestampDisplay timestamp={log.timestamp} onToggle={setTimestampExpanded} />
                                        </div>
                                    </div>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
                {!isMobile && (
                    <div className="flex items-stretch md:items-center gap-3.5 md:gap-3 relative flex-[0_0_auto] min-w-0">
                        <div className="hidden md:flex flex-col md:flex-row items-start md:items-center md:gap-2.5 gap-1 flex-shrink-0">
                            <div className="relative w-[60px] md:w-[100px] font-text-xs-leading-4-semibold font-semibold text-[10px] md:text-[length:var(--text-xs-leading-4-semibold-font-size)] text-[var(--tailwind-colors-rdns-600)] text-left md:text-center tracking-wide leading-4 md:leading-[var(--text-xs-leading-4-semibold-line-height)] uppercase order-0 md:order-1">
                                {protocolLabel}
                            </div>
                            <Badge className={`order-1 md:order-0 inline-flex items-center justify-center px-2 py-0.5 md:pt-[var(--tailwind-primitives-padding-p-0-5)] md:pr-[var(--tailwind-primitives-padding-p-2-5)] md:pb-[var(--tailwind-primitives-padding-p-0-5)] md:pl-[var(--tailwind-primitives-padding-p-2-5)] bg-[var(--tailwind-colors-red-600)] rounded border-0 h-5 md:h-auto ${!isBlocked ? 'opacity-0 pointer-events-none select-none' : ''}`} aria-hidden={!isBlocked}>
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
                            <TimestampDisplay timestamp={log.timestamp} onToggle={setTimestampExpanded} />
                        </div>
                        {renderQuickRuleButton("flex items-center justify-center")}
                    </div>
                )}
            </div>
        </div>
    );
};

interface TimestampDisplayProps { timestamp?: string; onToggle?: (expanded: boolean) => void }

const TimestampDisplay = ({ timestamp, onToggle }: TimestampDisplayProps) => {
    const [expanded, setExpanded] = useState(false);
    if (!timestamp) return null;
    const date = parseISO(timestamp);
    const relative = formatDistanceToNow(date, { addSuffix: true });
    const absolute = format(date, "MMMM d, yyyy 'at' hh:mm:ss a");
    return (
        <button
            type="button"
            aria-expanded={expanded}
            onClick={() => setExpanded(e => { const next = !e; onToggle?.(next); return next; })}
            className={`group relative w-fit font-text-xs-leading-5-normal font-[number:var(--text-xs-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-100)] text-[length:var(--text-xs-leading-5-normal-font-size)] tracking-[var(--text-xs-leading-5-normal-letter-spacing)] leading-[var(--text-xs-leading-5-normal-line-height)] whitespace-nowrap [font-style:var(--text-xs-leading-5-normal-font-style)] inline-flex items-center gap-1 focus:outline-none cursor-pointer select-text transition-all duration-300 ease-out ${expanded ? 'mt-0.5' : ''}`}
            title={expanded ? 'Show relative time' : 'Show full timestamp'}
        >
            <Clock className="w-3 h-3 opacity-60 group-hover:opacity-100 transition-opacity" />
            <span className="relative inline-flex min-h-[20px]">
                <span
                    className={`whitespace-nowrap transition-all duration-200 ease-out ${expanded ? 'opacity-0 -translate-y-1 absolute left-0 top-0 pointer-events-none' : 'opacity-100 translate-y-0 relative'}`}
                >
                    {relative}
                </span>
                <span
                    className={`whitespace-nowrap transition-all duration-200 ease-out ${expanded ? 'opacity-100 translate-y-0 relative' : 'opacity-0 translate-y-1 absolute left-0 top-0 pointer-events-none'}`}
                >
                    {absolute}
                </span>
            </span>
        </button>
    );
};

export default QueryLogCard;
