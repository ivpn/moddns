import { describe, expect, it } from 'vitest';
import type { ErrorEvent } from '@sentry/react';
import { createPrivacyHooks, scrubBreadcrumb, scrubTransaction, scrubUrl, type TransactionEvent } from '@/lib/sentryPrivacy';

describe('scrubUrl', () => {
    it('replaces the profile id in API paths and keeps the rest', () => {
        expect(scrubUrl('https://api.example.test/api/v1/profiles/a8h1bcy6s7/statistics?timespan=LAST_7_DAYS')).toBe(
            'https://api.example.test/api/v1/profiles/:profile_id/statistics?timespan=LAST_7_DAYS',
        );
        expect(scrubUrl('/api/v1/profiles/abc123')).toBe('/api/v1/profiles/:profile_id');
    });

    it('filters device and profile ids in query strings', () => {
        expect(scrubUrl('/api/v1/profiles/p1/logs?device_id=laptop&limit=25&profile_id=p1')).toBe(
            '/api/v1/profiles/:profile_id/logs?device_id=[Filtered]&limit=25&profile_id=[Filtered]',
        );
    });

    it('replaces device path segments and DoH profile paths', () => {
        expect(scrubUrl('/devices/phone-1/rename')).toBe('/devices/:device_id/rename');
        expect(scrubUrl('https://dns.example.test/dns-query/a8h1bcy6s7')).toBe('https://dns.example.test/dns-query/:profile_id');
    });

    it('leaves URLs without identifiers untouched, including the hash', () => {
        expect(scrubUrl('/statistics?range=30d#top')).toBe('/statistics?range=30d#top');
        expect(scrubUrl('')).toBe('');
    });
});

describe('scrubBreadcrumb', () => {
    it('scrubs fetch, xhr and navigation breadcrumbs', () => {
        const fetchCrumb = scrubBreadcrumb({ category: 'fetch', data: { url: '/api/v1/profiles/p1/logs/devices', method: 'GET', status_code: 200 } });
        expect(fetchCrumb.data).toEqual({ url: '/api/v1/profiles/:profile_id/logs/devices', method: 'GET', status_code: 200 });
        const nav = scrubBreadcrumb({ category: 'navigation', data: { from: '/profiles/p1/x', to: '/statistics?device_id=d' } });
        expect(nav.data).toEqual({ from: '/profiles/:profile_id/x', to: '/statistics?device_id=[Filtered]' });
    });

    it('scrubs the message', () => {
        expect(scrubBreadcrumb({ message: 'GET /api/v1/profiles/p1' }).message).toBe('GET /api/v1/profiles/:profile_id');
    });

    it('tolerates a breadcrumb without data', () => {
        expect(scrubBreadcrumb({ category: 'ui.click' })).toMatchObject({ category: 'ui.click' });
    });
});

describe('scrubTransaction', () => {
    it('scrubs the name, request URL, span descriptions and span data', () => {
        const event = {
            type: 'transaction',
            transaction: '/api/v1/profiles/p1/statistics',
            request: { url: 'https://app.example.test/statistics?device_id=d1' },
            spans: [
                {
                    description: 'GET /api/v1/profiles/p1/logs/top?kind=blocked',
                    span_id: 's',
                    trace_id: 't',
                    start_timestamp: 0,
                    data: { url: 'https://api.example.test/api/v1/profiles/p1/logs/top', 'http.query': '?device_id=d1' },
                },
            ],
            breadcrumbs: [{ category: 'fetch', data: { url: '/api/v1/profiles/p1' } }],
            contexts: { trace: { trace_id: 't', span_id: 's', data: { 'url.full': 'https://x.test/profiles/p1' } } },
        } as unknown as TransactionEvent;
        const out = scrubTransaction(event);
        const text = JSON.stringify(out);
        expect(text).not.toContain('p1');
        expect(text).not.toContain('"d1"');
        expect(out.transaction).toBe('/api/v1/profiles/:profile_id/statistics');
        expect(out.spans?.[0].description).toBe('GET /api/v1/profiles/:profile_id/logs/top?kind=blocked');
    });
});

describe('consent gating', () => {
    const tx = { type: 'transaction', transaction: '/statistics' } as unknown as TransactionEvent;
    const err = { type: undefined, message: 'boom' } as unknown as ErrorEvent;

    it('drops errors and transactions and samples traces at zero without consent', () => {
        const hooks = createPrivacyHooks(() => false);
        expect(hooks.beforeSend(err)).toBeNull();
        expect(hooks.beforeSendTransaction(tx)).toBeNull();
        expect(hooks.tracesSampler()).toBe(0);
    });

    it('sends scrubbed events and samples traces with consent', () => {
        const hooks = createPrivacyHooks(() => true);
        expect(hooks.beforeSend(err)).toMatchObject({ message: 'boom' });
        expect(hooks.beforeSendTransaction({ ...tx, transaction: '/profiles/p9/x' } as TransactionEvent)?.transaction).toBe('/profiles/:profile_id/x');
        expect(hooks.tracesSampler()).toBe(1);
    });

    it('re-reads consent on every call so revoking takes effect at once', () => {
        let consent = true;
        const hooks = createPrivacyHooks(() => consent);
        expect(hooks.tracesSampler()).toBe(1);
        consent = false;
        expect(hooks.tracesSampler()).toBe(0);
        expect(hooks.beforeSendTransaction(tx)).toBeNull();
    });

    it('scrubs breadcrumbs regardless of consent', () => {
        const hooks = createPrivacyHooks(() => false);
        expect(hooks.beforeBreadcrumb({ message: '/profiles/p1' }).message).toBe('/profiles/:profile_id');
    });
});
