import { describe, it, expect } from 'vitest';
import { formatReasons } from '@/lib/formatReasons';

const blocklistNames = { 'hagezi-tif': 'HaGeZi TIF', 'x': 'Blocklist X' };
const serviceNames = { 'tiktok': 'TikTok', 'y': 'Service Y' };

describe('formatReasons', () => {
    it('maps a specific blocklist id to a resolved name', () => {
        // tableRef: logs-reason-display-behaviour #1
        expect(formatReasons(['blocklist: hagezi-tif'], blocklistNames, serviceNames)).toEqual([
            { kind: 'blocklist', label: 'Blocklist: HaGeZi TIF' },
        ]);
    });

    it('renders a generic Blocklist chip when only the generic token is present', () => {
        // tableRef: logs-reason-display-behaviour #2
        expect(formatReasons(['blocklists'], blocklistNames, serviceNames)).toEqual([
            { kind: 'blocklist', label: 'Blocklist' },
        ]);
    });

    it('collapses generic + specific blocklist into the specific chip', () => {
        // tableRef: logs-reason-display-behaviour #3
        expect(formatReasons(['blocklists', 'blocklist: x'], blocklistNames, serviceNames)).toEqual([
            { kind: 'blocklist', label: 'Blocklist: Blocklist X' },
        ]);
    });

    it('folds the subdomain rule into the blocklist chip as a qualifier', () => {
        // tableRef: logs-reason-display-behaviour #4
        expect(
            formatReasons(['blocklist: x', 'blocklists_subdomains_rule'], blocklistNames, serviceNames)
        ).toEqual([{ kind: 'blocklist', label: 'Blocklist: Blocklist X (subdomain)' }]);
    });

    it('maps a specific service id to a resolved name', () => {
        // tableRef: logs-reason-display-behaviour #5
        expect(formatReasons(['service: tiktok'], blocklistNames, serviceNames)).toEqual([
            { kind: 'service', label: 'Service: TikTok' },
        ]);
    });

    it('renders a generic Service chip when only the generic token is present', () => {
        // tableRef: logs-reason-display-behaviour #6
        expect(formatReasons(['services'], blocklistNames, serviceNames)).toEqual([
            { kind: 'service', label: 'Service' },
        ]);
    });

    it('collapses generic + specific service into the specific chip', () => {
        // tableRef: logs-reason-display-behaviour #7
        expect(formatReasons(['services', 'service: y'], blocklistNames, serviceNames)).toEqual([
            { kind: 'service', label: 'Service: Service Y' },
        ]);
    });

    it('maps custom_rules to a Custom rule chip', () => {
        // tableRef: logs-reason-display-behaviour #8
        expect(formatReasons(['custom_rules'], blocklistNames, serviceNames)).toEqual([
            { kind: 'custom_rule', label: 'Custom rule' },
        ]);
    });

    it('maps default_rule to a Default rule chip', () => {
        // tableRef: logs-reason-display-behaviour #9
        expect(formatReasons(['default_rule'], blocklistNames, serviceNames)).toEqual([
            { kind: 'default', label: 'Default rule' },
        ]);
    });

    it('renders nothing for empty input', () => {
        // tableRef: logs-reason-display-behaviour #10
        expect(formatReasons([], blocklistNames, serviceNames)).toEqual([]);
    });

    it('renders multiple same-tier chips in a stable order (blocklist then service)', () => {
        // tableRef: logs-reason-display-behaviour #11
        expect(
            formatReasons(['service: y', 'blocklist: x'], blocklistNames, serviceNames)
        ).toEqual([
            { kind: 'blocklist', label: 'Blocklist: Blocklist X' },
            { kind: 'service', label: 'Service: Service Y' },
        ]);
    });

    it('falls back to the raw id when the name map has no entry', () => {
        // tableRef: logs-reason-display-behaviour #12
        expect(formatReasons(['blocklist: unknown-id'], blocklistNames, serviceNames)).toEqual([
            { kind: 'blocklist', label: 'Blocklist: unknown-id' },
        ]);
    });

    it('works without name maps, falling back to raw ids', () => {
        // tableRef: logs-reason-display-behaviour #12
        expect(formatReasons(['service: some-svc'])).toEqual([
            { kind: 'service', label: 'Service: some-svc' },
        ]);
    });
});
