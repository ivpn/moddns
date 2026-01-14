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

describe('QueryLogCard truncation interactions', () => {
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
        // Tooltip still present wrapping element; hover should not change content
        fireEvent.mouseEnter(fullEl);
        expect(fullEl).toHaveTextContent(deviceId);
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

    test('mobile tap expands truncated domain (threshold 65)', () => {
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
        const truncatedDomainBtn = screen.getByTestId('querylog-domain-truncated');
        expect(truncatedDomainBtn).toBeInTheDocument();
        // Verify it contains ellipsis at end
        expect(truncatedDomainBtn.textContent).toMatch(/…$/);
        fireEvent.click(truncatedDomainBtn);
        const fullDomainSpan = screen.getByTestId('querylog-domain-full');
        expect(fullDomainSpan).toHaveTextContent(longDomain);
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

