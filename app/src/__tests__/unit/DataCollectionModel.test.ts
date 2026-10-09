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

const base: DataCollectionState = { stats: false, logs: false, statsRetention: '30d', domains: true, ips: false, retention: '1h' };
const states: Record<StateKey, DataCollectionState> = {
    S0: { ...base },
    S1: { ...base, stats: true },
    S2: { ...base, stats: true, logs: true },
    S3: { ...base, logs: true },
};

const profile = (logs: object, statistics: object) =>
    ({ settings: { logs, statistics } }) as unknown as ModelProfile;

describe('fromProfile', () => {
    it('maps the settings fields to the two sources', () => {
        // tableRef: statistics-behaviour #L1
        expect(stateKey(fromProfile(profile({ enabled: false }, { enabled: false })))).toBe('S0');
        // tableRef: statistics-behaviour #L2
        expect(stateKey(fromProfile(profile({ enabled: false }, { enabled: true })))).toBe('S1');
        // tableRef: statistics-behaviour #L3
        expect(stateKey(fromProfile(profile({ enabled: true }, { enabled: true })))).toBe('S2');
        // tableRef: statistics-behaviour #L4
        expect(stateKey(fromProfile(profile({ enabled: true }, { enabled: false })))).toBe('S3');
    });

    it('renders API-set logs-only with only Query logs on and never repairs it', () => {
        // tableRef: statistics-behaviour #L5
        const s = fromProfile(profile({ enabled: true }, { enabled: false }));
        expect(s.logs).toBe(true);
        expect(s.stats).toBe(false);
    });

    it('reads the statistics retention, an unknown one as 30 days', () => {
        // tableRef: statistics-behaviour #C5
        expect(fromProfile(profile({}, { retention: '1y' })).statsRetention).toBe('1y');
        expect(fromProfile(profile({}, { retention: 'x' })).statsRetention).toBe('30d');
    });

    it('defaults missing fields (domains on, IPs off, 1 hour, 30 days)', () => {
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

    it('ignores sub-options staged under an unchecked Query logs card', () => {
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
        expect(transitionFor(states.S0, states.S2)?.label).toBe('Turn on statistics and query logs');
        expect(transitionFor(states.S0, states.S3)?.label).toBe('Turn on query logs');
        expect(transitionFor(states.S3, states.S2)?.label).toBe('Turn on statistics');
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

    it('writes both enabled paths on a state change', () => {
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

    it('drops staged sub-options of an unchecked card', () => {
        expect(asPairs(states.S2, { ...states.S0, ips: true, statsRetention: '1y' })).toEqual([
            ['/settings/statistics/enabled', false],
            ['/settings/logs/enabled', false],
        ]);
    });

    it('stages the statistics retention with the turn-on, in the same request', () => {
        // tableRef: statistics-behaviour #T16, #L6
        expect(asPairs(states.S0, { ...states.S1, statsRetention: '1y' })).toEqual([
            ['/settings/statistics/enabled', true],
            ['/settings/logs/enabled', false],
            ['/settings/statistics/retention', '1y'],
        ]);
    });

    it('sends only the retention path on a retention-only change', () => {
        // tableRef: statistics-behaviour #T14, #T15
        expect(asPairs(states.S1, { ...states.S1, statsRetention: '90d' })).toEqual([['/settings/statistics/retention', '90d']]);
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
        expect(transitionFor(states.S0, { ...states.S1, statsRetention: '1y' })?.note).toBe(
            "modDNS will start counting this profile's queries per device and keep the counts for 1 year. No domains or addresses are stored. The first counts appear at the next quarter hour.",
        );
        expect(transitionFor(states.S3, { ...states.S1, statsRetention: '90d' })?.dialog?.body).toContain('counting queries per device for 90 days');
        expect(transitionFor(states.S3, { ...states.S2, statsRetention: '90d' })?.note).toBe('modDNS will also keep counts per device for 90 days. The first counts appear at the next quarter hour.');
    });

    it.each([['S0', 'S1'], ['S0', 'S2'], ['S3', 'S1'], ['S3', 'S2']] as const)('ends the %s to %s note with the first-counts line', (from, to) => {
        // tableRef: statistics-behaviour #C21
        expect(transitionFor(states[from], states[to])?.note).toMatch(/ The first counts appear at the next quarter hour\.$/);
    });

    it.each([['S0', 'S3'], ['S1', 'S2'], ['S1', 'S3']] as const)('leaves the %s to %s note without it', (from, to) => {
        // tableRef: statistics-behaviour #C21
        expect(transitionFor(states[from], states[to])?.note).not.toContain('first counts');
    });
});

describe('statistics retention changes', () => {
    const kept = (r: DataCollectionState['statsRetention']) => ({ ...states.S1, statsRetention: r });

    it('raising alone is T14: a note, no dialog, labelled by the new value', () => {
        // tableRef: statistics-behaviour #T14, #C23, #C16
        const t = transitionFor(kept('30d'), kept('1y'));
        expect(t).toMatchObject({
            id: 'T14',
            label: 'Keep for 1 year',
            toast: 'Statistics are now kept for 1 year.',
            note: "modDNS will keep this profile's counts for up to 1 year.",
        });
        expect(t?.dialog).toBeUndefined();
    });

    it('lowering alone is T15 with the destructive dialog', () => {
        // tableRef: statistics-behaviour #T15, #C24, #C16
        const t = transitionFor(kept('1y'), kept('90d'));
        expect(t).toMatchObject({ id: 'T15', toast: 'Statistics are now kept for 90 days. Older counts deleted.' });
        expect(t?.dialog).toEqual({
            title: 'Keep statistics for 90 days?',
            body: 'Counts older than 90 days will be permanently deleted. This action cannot be undone.',
            confirm: 'Delete older counts',
        });
    });

    it('adds the lowering sentence to a state change that has its own dialog', () => {
        // tableRef: statistics-behaviour #T15
        const t = transitionFor({ ...states.S2, statsRetention: '1y' }, { ...states.S1, statsRetention: '30d' });
        expect(t?.id).toBe('T8');
        expect(t?.dialog?.body).toBe(
            'All query logs for this profile will be permanently deleted. Statistics stay on. Counts older than 30 days will be permanently deleted. This action cannot be undone.',
        );
    });

    it('a lowering with a non-deleting change still asks for confirmation', () => {
        // tableRef: statistics-behaviour #T15
        const t = transitionFor({ ...states.S1, statsRetention: '1y' }, { ...states.S2, statsRetention: '30d' });
        expect(t?.id).toBe('T5');
        expect(t?.dialog?.title).toBe('Keep statistics for 30 days?');
    });

    it('appends the raise note to the state change note', () => {
        // tableRef: statistics-behaviour #T14
        expect(transitionFor(states.S1, { ...states.S2, statsRetention: '90d' })?.note).toBe(
            "modDNS will also record each query for 1 hour. modDNS will keep this profile's counts for up to 90 days.",
        );
    });

    it('with logs sub-options in the same save it reads as T13 with the retention parts', () => {
        // tableRef: statistics-behaviour #T13, #T14
        const t = transitionFor(states.S2, { ...states.S2, ips: true, statsRetention: '1y' });
        expect(t).toMatchObject({ id: 'T13', label: 'Save changes', toast: 'Data collection updated.', retention: 'raise' });
    });

    it('ignores a retention staged while statistics stay off', () => {
        // tableRef: statistics-behaviour #L6
        expect(transitionFor(states.S3, { ...states.S3, statsRetention: '1y' })).toBeNull();
    });

    it('turning statistics on with a retention is the plain turn-on, named with the pending value', () => {
        // tableRef: statistics-behaviour #T16
        const t = transitionFor(states.S0, { ...states.S1, statsRetention: '90d' });
        expect(t?.id).toBe('T1');
        expect(t?.dialog).toBeUndefined();
        expect(t?.note).toContain('keep the counts for 90 days');
    });
});
