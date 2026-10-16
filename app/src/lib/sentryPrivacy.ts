// Privacy rules for the Sentry integration.
//
// Tracing and session replay run only with the account's error_reports_consent, and no URL that
// leaves the browser carries a profile or device identifier.

import type { Breadcrumb, BrowserOptions, ErrorEvent } from "@sentry/react";

export type TransactionEvent = Parameters<NonNullable<BrowserOptions["beforeSendTransaction"]>>[0];

const FILTERED = "[Filtered]";
const ID_PARAMS = new Set(["device_id", "device", "profile_id", "profile", "id"]);

/** Replaces profile and device identifiers in a URL, path or query string. */
export function scrubUrl(url: string): string {
    if (!url) return url;
    const [beforeHash, hash] = url.split("#", 2);
    const [path, query] = beforeHash.split("?", 2);
    const cleanPath = path
        .replace(/(\/profiles\/)[^/?#\s]+/gi, "$1:profile_id")
        .replace(/(\/devices\/)[^/?#\s]+/gi, "$1:device_id")
        .replace(/(\/dns-query\/)[^/?#\s]+/gi, "$1:profile_id");
    let out = cleanPath;
    if (query !== undefined) {
        const params = query.split("&").map(pair => {
            const [k, v] = pair.split("=", 2);
            return v !== undefined && ID_PARAMS.has(decodeURIComponent(k).toLowerCase()) ? `${k}=${FILTERED}` : pair;
        });
        out += `?${params.join("&")}`;
    }
    return hash !== undefined ? `${out}#${hash}` : out;
}

function scrubValue(value: unknown): unknown {
    return typeof value === "string" ? scrubUrl(value) : value;
}

const URL_KEYS = ["url", "from", "to", "http.url", "http.query", "url.full", "url.path", "http.target", "path"];

function scrubData<T extends Record<string, unknown> | undefined>(data: T): T {
    if (!data) return data;
    const out: Record<string, unknown> = { ...data };
    for (const key of URL_KEYS) if (key in out) out[key] = scrubValue(out[key]);
    return out as T;
}

export function scrubBreadcrumb(crumb: Breadcrumb): Breadcrumb {
    return {
        ...crumb,
        message: crumb.message ? scrubUrl(crumb.message) : crumb.message,
        data: scrubData(crumb.data),
    };
}

export function scrubTransaction(event: TransactionEvent): TransactionEvent {
    return {
        ...event,
        transaction: event.transaction ? scrubUrl(event.transaction) : event.transaction,
        request: event.request?.url ? { ...event.request, url: scrubUrl(event.request.url) } : event.request,
        breadcrumbs: event.breadcrumbs?.map(scrubBreadcrumb),
        spans: event.spans?.map(span => ({
            ...span,
            description: span.description ? scrubUrl(span.description) : span.description,
            data: scrubData(span.data),
        })),
        contexts: event.contexts?.trace
            ? { ...event.contexts, trace: { ...event.contexts.trace, data: scrubData(event.contexts.trace.data) } }
            : event.contexts,
    };
}

export function scrubErrorEvent(event: ErrorEvent): ErrorEvent {
    return {
        ...event,
        request: event.request?.url ? { ...event.request, url: scrubUrl(event.request.url) } : event.request,
        breadcrumbs: event.breadcrumbs?.map(scrubBreadcrumb),
    };
}

/** The Sentry hooks and samplers, parameterised on the consent check so they can be tested. */
export function createPrivacyHooks(hasConsent: () => boolean) {
    return {
        beforeSend: (event: ErrorEvent): ErrorEvent | null => (hasConsent() ? scrubErrorEvent(event) : null),
        beforeSendTransaction: (event: TransactionEvent): TransactionEvent | null =>
            hasConsent() ? scrubTransaction(event) : null,
        beforeBreadcrumb: (crumb: Breadcrumb): Breadcrumb => scrubBreadcrumb(crumb),
        tracesSampler: (): number => (hasConsent() ? 1 : 0),
    };
}
