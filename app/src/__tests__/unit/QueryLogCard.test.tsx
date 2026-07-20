import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import QueryLogCard from '@/pages/logs/QueryLogCard';
import { describe, test, expect, beforeEach, vi } from 'vitest';
import type { ModelQueryLog } from '@/api/client';

// Helper to stub matchMedia with basic capability flags
function stubDesktopMatchMedia(isDesktop: boolean) {
    (window as unknown as { matchMedia: (query: string) => MediaQueryList }).matchMedia = (query: string) => {
        const matchesWidth = /min-width:1024px/.test(query);
        const matchesHoverFine = /(hover:hover)|(pointer:fine)/.test(query);
        const matches = isDesktop && (matchesWidth || matchesHoverFine);
        const mq: MediaQueryList = {
            matches,
            media: query,
            onchange: null,
            addEventListener: () => { },
            removeEventListener: () => { },
            dispatchEvent: () => false,
            // Deprecated listeners still included for compatibility
            addListener: () => { },
            removeListener: () => { }
        };
        return mq;
    };
}

describe('QueryLogCard truncation display', () => {
    beforeEach(() => {
        // Reset viewport width
        // Override viewport width for desktop simulation
        (window as unknown as { innerWidth: number }).innerWidth = 1440;
    });

    test('desktop shows full 16-char device id with no ellipsis', () => {
        stubDesktopMatchMedia(true);
        const deviceId = 'device-id-123456'; // 16 chars example
        const log: ModelQueryLog = {
            profile_id: 'p1',
            timestamp: new Date().toISOString(),
            status: 'processed',
            protocol: 'dns',
            device_id: deviceId,
            client_ip: '10.0.0.1',
            dns_request: { domain: 'example.com' }
        };
        render(<QueryLogCard log={log} />);
        const fullEl = screen.getByTestId('querylog-device-id-full');
        expect(fullEl).toHaveTextContent(deviceId);
        expect(fullEl.textContent).toHaveLength(deviceId.length);
        expect(fullEl.textContent?.endsWith('…')).toBeFalsy();
    });

    test('desktop domain display strips trailing dot', () => {
        stubDesktopMatchMedia(true);
        const log: ModelQueryLog = {
            profile_id: 'p-dot',
            timestamp: new Date().toISOString(),
            status: 'blocked',
            protocol: 'dns',
            device_id: 'device-with-dot',
            client_ip: '10.0.0.55',
            dns_request: { domain: 'blocked.example.com.' }
        };
        render(<QueryLogCard log={log} />);
        const domainSpan = screen.getByTestId('querylog-domain-full');
        expect(domainSpan).toHaveTextContent('blocked.example.com');
        expect(domainSpan).not.toHaveTextContent(/\.$/);
    });

    test('mobile renders a static truncated domain span (no tap-to-reveal)', () => {
        stubDesktopMatchMedia(false);
        // Override viewport width for mobile simulation
        (window as unknown as { innerWidth: number }).innerWidth = 375;
        // Craft a domain exceeding current DOMAIN_TRUNCATE_THRESHOLD (65) to trigger truncation.
        const longDomain = 'sub.sub.sub.really-long-domain-name-for-testing.example.reallyreallylongsegment.test';
        const log: ModelQueryLog = {
            profile_id: 'p2',
            timestamp: new Date().toISOString(),
            status: 'processed',
            protocol: 'dns',
            device_id: 'short-id',
            client_ip: '10.0.0.2',
            dns_request: { domain: longDomain }
        };
        render(<QueryLogCard log={log} />);
        const truncatedDomain = screen.getByTestId('querylog-domain-truncated');
        expect(truncatedDomain).toBeInTheDocument();
        // Static truncated text ends with an ellipsis; it is a plain span (not a button).
        expect(truncatedDomain.textContent).toMatch(/…$/);
        expect(truncatedDomain.tagName).toBe('SPAN');
    });
});

describe('QueryLogCard whole-card expansion', () => {
    beforeEach(() => {
        (window as unknown as { innerWidth: number }).innerWidth = 1440;
        stubDesktopMatchMedia(true);
    });

    const baseLog: ModelQueryLog = {
        profile_id: 'p-exp',
        timestamp: '2026-06-15T10:20:30.000Z',
        status: 'processed',
        protocol: 'dns',
        device_id: 'expand-device',
        client_ip: '10.0.0.9',
        dns_request: { domain: 'expand.example.com', query_type: 'A', response_code: 'NOERROR', dnssec: true }
    };

    test('renders the whole-card toggle', () => {
        render(<QueryLogCard log={baseLog} />);
        expect(screen.getByTestId('querylog-card-toggle')).toBeInTheDocument();
    });

    test('clicking the toggle flips the expanded panel state', () => {
        render(<QueryLogCard log={baseLog} />);
        const toggle = screen.getByTestId('querylog-card-toggle');
        const panel = screen.getByTestId('querylog-expanded-panel');
        expect(panel).toHaveAttribute('data-expanded', 'false');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        fireEvent.click(toggle);
        expect(panel).toHaveAttribute('data-expanded', 'true');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
    });

    test('expanded panel shows the detail grid with protocol and timestamp', () => {
        render(<QueryLogCard log={baseLog} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        expect(screen.getByTestId('querylog-detail-grid')).toBeInTheDocument();
        expect(screen.getByTestId('querylog-detail-protocol')).toHaveTextContent('DNS');
        expect(screen.getByTestId('querylog-detail-timestamp')).toBeInTheDocument();
    });

    test('row with reasons renders the reasons block', () => {
        const log: ModelQueryLog = {
            ...baseLog,
            status: 'blocked',
            reasons: ['blocklist: some-blocklist-id']
        };
        render(<QueryLogCard log={log} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        expect(screen.getByTestId('querylog-reasons')).toBeInTheDocument();
    });

    test('row without reasons omits the reasons block but still expands', () => {
        render(<QueryLogCard log={baseLog} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        expect(screen.getByTestId('querylog-detail-grid')).toBeInTheDocument();
        expect(screen.queryByTestId('querylog-reasons')).not.toBeInTheDocument();
    });

    test('domain-logging-disabled row is still expandable and shows a placeholder', () => {
        const log: ModelQueryLog = {
            ...baseLog,
            dns_request: undefined as unknown as ModelQueryLog['dns_request']
        };
        render(<QueryLogCard log={log} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        expect(screen.getByTestId('querylog-detail-domain')).toHaveTextContent('Domain logging disabled');
    });

    test('there is no visible chevron indicator', () => {
        render(<QueryLogCard log={baseLog} />);
        expect(screen.queryByTestId('querylog-expand-indicator')).not.toBeInTheDocument();
    });

    test('onExpand fires only when expanding (not when collapsing)', () => {
        const onExpand = vi.fn();
        render(<QueryLogCard log={baseLog} onExpand={onExpand} />);
        const toggle = screen.getByTestId('querylog-card-toggle');
        fireEvent.click(toggle); // expand
        expect(onExpand).toHaveBeenCalledTimes(1);
        fireEvent.click(toggle); // collapse
        expect(onExpand).toHaveBeenCalledTimes(1);
    });

    test('shows the DNSSEC badge on the collapsed row when validated', () => {
        render(<QueryLogCard log={baseLog} />); // baseLog has dns_request.dnssec === true
        expect(screen.getByTestId('querylog-dnssec-badge')).toHaveTextContent('DNSSEC');
    });

    test('omits the DNSSEC badge when neither validated nor failed', () => {
        const log: ModelQueryLog = {
            ...baseLog,
            dns_request: { ...baseLog.dns_request, dnssec: false }
        };
        render(<QueryLogCard log={log} />);
        expect(screen.queryByTestId('querylog-dnssec-badge')).not.toBeInTheDocument();
    });

    test('shows a red (failed) DNSSEC badge when validation failed', () => {
        const log: ModelQueryLog = {
            ...baseLog,
            status: 'processed',
            dns_request: { ...baseLog.dns_request, dnssec: false, response_code: 'SERVFAIL' },
            reasons: ['dnssec_failed'],
        };
        render(<QueryLogCard log={log} />);
        const badge = screen.getByTestId('querylog-dnssec-badge');
        expect(badge).toHaveTextContent('DNSSEC');
        expect(badge).toHaveAttribute('data-dnssec', 'failed');
    });

    test('labels the reason "Block reason" for a DNSSEC-failed row (not "Allow reason")', () => {
        const log: ModelQueryLog = {
            ...baseLog,
            status: 'processed',
            dns_request: { ...baseLog.dns_request, dnssec: false, response_code: 'SERVFAIL' },
            reasons: ['dnssec_failed'],
        };
        render(<QueryLogCard log={log} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        const reasons = screen.getByTestId('querylog-reasons');
        expect(reasons).toHaveTextContent('Block reason');
        expect(reasons).not.toHaveTextContent('Allow reason');
    });

    test('DNSSEC detail field distinguishes the three states', () => {
        const detailText = (log: ModelQueryLog) => {
            const { unmount } = render(<QueryLogCard log={log} />);
            fireEvent.click(screen.getByTestId('querylog-card-toggle'));
            const text = screen.getByTestId('querylog-detail-dnssec').textContent;
            unmount();
            return text;
        };
        // validated
        expect(detailText(baseLog)).toBe('Validated');
        // unsigned (dnssec false, no failure reason)
        expect(detailText({ ...baseLog, dns_request: { ...baseLog.dns_request, dnssec: false } })).toBe('No DNSSEC');
        // failed (bogus)
        expect(detailText({
            ...baseLog,
            status: 'processed',
            dns_request: { ...baseLog.dns_request, dnssec: false, response_code: 'SERVFAIL' },
            reasons: ['dnssec_failed'],
        })).toBe('Validation failed');
    });
});

describe('QueryLogCard consolidation (issue #161)', () => {
    beforeEach(() => {
        (window as unknown as { innerWidth: number }).innerWidth = 1440;
        stubDesktopMatchMedia(true);
    });

    const memberA: ModelQueryLog = {
        profile_id: 'p-con',
        timestamp: '2026-06-15T10:20:32.000Z',
        status: 'processed',
        protocol: 'dns',
        device_id: 'con-device',
        client_ip: '10.0.0.9',
        dns_request: { domain: 'dup.example.com', query_type: 'A', response_code: 'NOERROR' },
    };
    const memberAAAA: ModelQueryLog = {
        ...memberA,
        timestamp: '2026-06-15T10:20:30.000Z',
        dns_request: { domain: 'dup.example.com', query_type: 'AAAA', response_code: 'NXDOMAIN' },
    };
    const group = {
        key: 'con-group',
        representative: memberA,
        count: 3,
        members: [memberA, memberAAAA, memberA],
        firstTimestamp: memberA.timestamp,
        lastTimestamp: memberAAAA.timestamp,
        queryTypes: ['A', 'AAAA'],
        responseCodes: ['NOERROR', 'NXDOMAIN'],
    };

    test('single-entry row (no group / count 1) shows no count badge', () => {
        render(<QueryLogCard log={memberA} />);
        expect(screen.queryByTestId('querylog-count-badge')).not.toBeInTheDocument();
        render(<QueryLogCard log={memberA} group={{ ...group, count: 1, members: [memberA], queryTypes: ['A'], responseCodes: ['NOERROR'] }} />);
        expect(screen.queryByTestId('querylog-count-badge')).not.toBeInTheDocument();
    });

    test('consolidated row shows a ×N count badge', () => {
        render(<QueryLogCard log={memberA} group={group} />);
        const badge = screen.getByTestId('querylog-count-badge');
        expect(badge).toHaveTextContent('×3');
        expect(badge).toHaveAttribute('data-count', '3');
    });

    test('expanded panel aggregates query types, response codes, occurrences and a time range', () => {
        render(<QueryLogCard log={memberA} group={group} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        expect(screen.getByTestId('querylog-detail-query-type')).toHaveTextContent('A, AAAA');
        expect(screen.getByTestId('querylog-detail-response-code')).toHaveTextContent('NOERROR, NXDOMAIN');
        expect(screen.getByTestId('querylog-detail-occurrences')).toHaveTextContent('3');
        // group spans 2s (10:20:30 → 10:20:32) → a time RANGE with an en dash and "Time range" label.
        expect(screen.getByTestId('querylog-detail-timestamp').textContent).toMatch(/–/);
        expect(screen.getByText('Time range')).toBeInTheDocument();
    });

    test('a group whose members share the same second shows a single "Time", not a range', () => {
        // A + AAAA fired back-to-back: same second, differing only in milliseconds.
        const sameSecondGroup = {
            ...group,
            firstTimestamp: '2026-06-15T10:20:32.480Z',
            lastTimestamp: '2026-06-15T10:20:32.010Z',
        };
        render(<QueryLogCard log={{ ...memberA, timestamp: '2026-06-15T10:20:32.480Z' }} group={sameSecondGroup} />);
        fireEvent.click(screen.getByTestId('querylog-card-toggle'));
        // No en dash → single time; label is the plain "Time" (exact, not "Time range").
        expect(screen.getByTestId('querylog-detail-timestamp').textContent).not.toMatch(/–/);
        expect(screen.getByText('Time')).toBeInTheDocument();
        expect(screen.queryByText('Time range')).not.toBeInTheDocument();
    });
});

describe('QueryLogCard quick rule button', () => {
    beforeEach(() => {
        (window as unknown as { innerWidth: number }).innerWidth = 1280;
        stubDesktopMatchMedia(true);
    });

    test('fires callback with normalized domain', () => {
        const onQuickRule = vi.fn();
        const log: ModelQueryLog = {
            profile_id: 'p3',
            timestamp: new Date().toISOString(),
            status: 'processed',
            protocol: 'dns',
            device_id: 'desktop-device',
            client_ip: '10.0.0.3',
            dns_request: { domain: 'Example.com.' }
        };
        render(<QueryLogCard log={log} onQuickRule={onQuickRule} />);
        const button = screen.getByTestId('logs-quick-rule-button');
        expect(button).toBeEnabled();
        fireEvent.click(button);
        expect(onQuickRule).toHaveBeenCalledTimes(1);
        expect(onQuickRule).toHaveBeenCalledWith('Example.com', 'denylist');
    });

    test('disables button when domain missing', () => {
        const onQuickRule = vi.fn();
        const log: ModelQueryLog = {
            profile_id: 'p4',
            timestamp: new Date().toISOString(),
            status: 'processed',
            protocol: 'dns',
            device_id: 'desktop-device',
            client_ip: '10.0.0.4',
            // Domain logging disabled
            dns_request: undefined as unknown as ModelQueryLog['dns_request']
        };
        render(<QueryLogCard log={log} onQuickRule={onQuickRule} />);
        const button = screen.getByTestId('logs-quick-rule-button');
        expect(button).toBeDisabled();
        fireEvent.click(button);
        expect(onQuickRule).not.toHaveBeenCalled();
    });
});
