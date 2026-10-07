import { describe, it, expect } from 'vitest';
import {
    buildUpdates,
    fromProfile,
    normalizeStatsRetention,
    profileStatsRetention,
    stateKey,
    statsRetentionWords,
    transitionFor,
    type DataCollectionState,
    type StateKey,
} from '@/components/data-collection/model';
import type { ModelProfile } from '@/api/client';

const base: DataCollectionState = { level: 'off', keep: false, domains: true, ips: false, retention: '1h' };
const states: Record<StateKey, DataCollectionState> = {
    S0: { ...base },
    S1: { ...base, level: 'stats', keep: true },
    S2: { ...base, level: 'logs', keep: true },
    S3: { ...base, level: 'logs', keep: false },
};

const profile = (logs: object, statistics: object) =>
    ({ settings: { logs, statistics } }) as unknown as ModelProfile;

describe('fromProfile', () => {
    it('maps the settings fields to the ladder', () => {
        // tableRef: statistics-behaviour #L1
        expect(stateKey(fromProfile(profile({ enabled: false }, { enabled: false })))).toBe('S0');
        // tableRef: statistics-behaviour #L2
        expect(stateKey(fromProfile(profile({ enabled: false }, { enabled: true })))).toBe('S1');
        // tableRef: statistics-behaviour #L3
        expect(stateKey(fromProfile(profile({ enabled: true }, { enabled: true })))).toBe('S2');
        // tableRef: statistics-behaviour #L4
        expect(stateKey(fromProfile(profile({ enabled: true }, { enabled: false })))).toBe('S3');
    });

    it('renders API-set logs-only as Query logs with the box unchecked and never repairs it', () => {
        // tableRef: statistics-behaviour #L5
        const s = fromProfile(profile({ enabled: true }, { enabled: false }));
        expect(s.level).toBe('logs');
        expect(s.keep).toBe(false);
    });

    it('defaults missing fields (domains on, IPs off, 1 hour)', () => {
        expect(fromProfile(profile({}, {}))).toEqual(base);
    });
});

describe('transitionFor', () => {
    const cases: [string, StateKey, StateKey, boolean, boolean][] = [
        // id, from, to, hasNote, hasDialog
        ['T1', 'S0', 'S1', true, false],
        ['T2', 'S0', 'S2', true, false],
        ['T3', 'S0', 'S3', true, false],
        ['T4', 'S1', 'S0', false, true],
        ['T5', 'S1', 'S2', true, false],
        ['T6', 'S1', 'S3', true, true],
        ['T7', 'S2', 'S0', false, true],
        ['T8', 'S2', 'S1', false, true],
        ['T9', 'S2', 'S3', false, true],
        ['T10', 'S3', 'S0', false, true],
        ['T11', 'S3', 'S1', true, true],
        ['T12', 'S3', 'S2', true, false],
    ];

    it.each(cases)('%s: %s -> %s', (id, from, to, hasNote, hasDialog) => {
        // tableRef: statistics-behaviour #T1..T12
        const t = transitionFor(states[from], states[to]);
        expect(t?.id).toBe(id);
        expect(!!t?.note).toBe(hasNote);
        expect(!!t?.dialog).toBe(hasDialog);
    });

    it('returns null when nothing changed', () => {
        expect(transitionFor(states.S2, { ...states.S2 })).toBeNull();
    });

    it('treats a sub-option-only change as T13 without a dialog or note', () => {
        // tableRef: statistics-behaviour #T13
        const t = transitionFor(states.S2, { ...states.S2, ips: true });
        expect(t).toMatchObject({ id: 'T13', label: 'Save changes' });
        expect(t?.dialog).toBeUndefined();
        expect(t?.note).toBeUndefined();
    });

    it('ignores sub-options staged under a non-logs level', () => {
        expect(transitionFor(states.S1, { ...states.S1, ips: true })).toBeNull();
    });

    it('uses the pending logs retention in words', () => {
        // tableRef: statistics-behaviour #C7
        expect(transitionFor(states.S0, { ...states.S3, retention: '1w' })?.note).toBe(
            'modDNS will record each query for 1 week. No counts are kept.',
        );
    });

    it('names the destructive result in each dialog', () => {
        // tableRef: statistics-behaviour #C8..C14
        expect(transitionFor(states.S1, states.S0)?.dialog).toEqual({
            title: 'Turn off statistics?',
            body: 'All statistics for this profile will be permanently deleted. This action cannot be undone.',
            confirm: 'Turn off and delete',
        });
        expect(transitionFor(states.S2, states.S1)?.dialog?.body).toContain('Statistics stay on.');
        expect(transitionFor(states.S2, states.S3)?.dialog?.body).toContain('Query logs stay on.');
        expect(transitionFor(states.S1, states.S3)?.dialog?.title).toBe('Switch to query logs only?');
        expect(transitionFor(states.S3, states.S1)?.dialog?.confirm).toBe('Switch and delete logs');
    });

    it('labels the Save button by the change', () => {
        // tableRef: statistics-behaviour #T21
        expect(transitionFor(states.S0, states.S1)?.label).toBe('Turn on statistics');
        expect(transitionFor(states.S0, states.S2)?.label).toBe('Turn on query logs');
        expect(transitionFor(states.S2, states.S0)?.label).toBe('Turn off');
        expect(transitionFor(states.S2, states.S1)?.label).toBe('Save changes');
    });

    it('never says restore or resume', () => {
        // tableRef: statistics-behaviour #C19
        for (const [, from, to] of cases) {
            const t = transitionFor(states[from], states[to]);
            const text = [t?.note, t?.dialog?.body, t?.toast].join(' ');
            expect(text).not.toMatch(/restore|resume/i);
        }
    });
});

describe('buildUpdates', () => {
    const asPairs = (a: DataCollectionState, b: DataCollectionState) =>
        buildUpdates(a, b).map(u => [u.path, u.value]);

    it('writes both enabled paths on a level change', () => {
        // tableRef: statistics-behaviour #T1
        expect(asPairs(states.S0, states.S1)).toEqual([
            ['/settings/statistics/enabled', true],
            ['/settings/logs/enabled', false],
        ]);
        // tableRef: statistics-behaviour #T7
        expect(asPairs(states.S2, states.S0)).toEqual([
            ['/settings/statistics/enabled', false],
            ['/settings/logs/enabled', false],
        ]);
        // tableRef: statistics-behaviour #T9
        expect(asPairs(states.S2, states.S3)).toEqual([
            ['/settings/statistics/enabled', false],
            ['/settings/logs/enabled', true],
        ]);
    });

    it('carries staged sub-options in the same request, before logs first turn on', () => {
        // tableRef: statistics-behaviour #L6
        expect(asPairs(states.S0, { ...states.S2, ips: true, domains: false, retention: '1w' })).toEqual([
            ['/settings/statistics/enabled', true],
            ['/settings/logs/enabled', true],
            ['/settings/logs/log_domains', false],
            ['/settings/logs/log_clients_ips', true],
            ['/settings/logs/retention', '1w'],
        ]);
    });

    it('sends only the changed sub-option on a T13 change', () => {
        // tableRef: statistics-behaviour #T13
        expect(asPairs(states.S2, { ...states.S2, ips: true })).toEqual([['/settings/logs/log_clients_ips', true]]);
    });

    it('drops staged sub-options when the target level is not Query logs', () => {
        expect(asPairs(states.S2, { ...states.S0, ips: true })).toEqual([
            ['/settings/statistics/enabled', false],
            ['/settings/logs/enabled', false],
        ]);
    });
});

describe('statistics retention', () => {
    it('reads an empty or unknown value as 30 days', () => {
        // tableRef: statistics-behaviour #P13
        expect(normalizeStatsRetention(undefined)).toBe('30d');
        expect(normalizeStatsRetention('')).toBe('30d');
        expect(normalizeStatsRetention('7d')).toBe('30d');
        expect(normalizeStatsRetention('90d')).toBe('90d');
        expect(normalizeStatsRetention('1y')).toBe('1y');
        expect(profileStatsRetention(profile({}, { retention: '1y' }))).toBe('1y');
        expect(profileStatsRetention(profile({}, {}))).toBe('30d');
        expect(profileStatsRetention(null)).toBe('30d');
    });

    it('words every retention', () => {
        expect(statsRetentionWords('30d')).toBe('30 days');
        expect(statsRetentionWords('90d')).toBe('90 days');
        expect(statsRetentionWords('1y')).toBe('1 year');
        expect(statsRetentionWords('nonsense')).toBe('30 days');
    });

    it('puts the live statistics retention into notes and dialogs', () => {
        // tableRef: statistics-behaviour #C7, #C14
        expect(transitionFor(states.S0, states.S1, '1 year')?.note).toBe(
            "modDNS will start counting this profile's queries per device and keep the counts for 1 year. No domains or addresses are stored.",
        );
        expect(transitionFor(states.S3, states.S1, '90 days')?.dialog?.body).toContain('count queries per device for 90 days'.replace('count', 'counting'));
        expect(transitionFor(states.S3, states.S2, '90 days')?.note).toBe('modDNS will also keep counts per device for 90 days.');
    });
});
