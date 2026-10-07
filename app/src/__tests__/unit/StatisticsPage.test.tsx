// @vitest-environment jsdom
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import Statistics from '@/pages/statistics/Statistics';
import { useAppStore } from '@/store/general';
import api from '@/api/api';
import type { ModelProfile } from '@/api/client';
import { createStatsResponse, devicesList, topBlocked, topClients, topResolved } from '../mocks/statisticsMocks';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            statisticsApi: { apiV1ProfilesIdStatisticsGet: vi.fn(), apiV1ProfilesIdStatisticsDelete: vi.fn() },
            queryLogsApi: {
                apiV1ProfilesIdLogsTopGet: vi.fn(),
                apiV1ProfilesIdLogsClientsGet: vi.fn(),
                apiV1ProfilesIdLogsDevicesGet: vi.fn(),
            },
            profilesApi: { apiV1ProfilesIdGet: vi.fn(), apiV1ProfilesIdPatch: vi.fn() },
        },
    },
}));

type Mock = ReturnType<typeof vi.fn>;
const statsGet = api.Client.statisticsApi.apiV1ProfilesIdStatisticsGet as unknown as Mock;
const topGet = api.Client.queryLogsApi.apiV1ProfilesIdLogsTopGet as unknown as Mock;
const clientsGet = api.Client.queryLogsApi.apiV1ProfilesIdLogsClientsGet as unknown as Mock;
const devicesGet = api.Client.queryLogsApi.apiV1ProfilesIdLogsDevicesGet as unknown as Mock;
const profileGet = api.Client.profilesApi.apiV1ProfilesIdGet as unknown as Mock;
const statsDelete = api.Client.statisticsApi.apiV1ProfilesIdStatisticsDelete as unknown as Mock;

type Cfg = 'OFF' | 'S' | 'SL' | 'SL-dom' | 'SL-ip' | 'L';

function mk(cfg: Cfg, id = 'p1', logsRetention = '1d'): ModelProfile {
    const logsOn = cfg !== 'OFF' && cfg !== 'S';
    const statsOn = cfg !== 'OFF' && cfg !== 'L';
    return {
        id,
        profile_id: id,
        account_id: 'a',
        name: id,
        settings: {
            logs: {
                enabled: logsOn,
                log_domains: cfg !== 'SL-dom',
                log_clients_ips: cfg === 'SL-ip' ? false : logsOn,
                retention: logsRetention,
            },
            statistics: { enabled: statsOn, retention: '30d' },
        },
    } as unknown as ModelProfile;
}

function Probe() {
    const l = useLocation();
    return <div data-testid="loc">{l.pathname + l.search}</div>;
}

function mount(p: ModelProfile, url = '/statistics') {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
    return render(
        <MemoryRouter initialEntries={[url]}>
            <Probe />
            <Routes>
                <Route path="/statistics" element={<Statistics profiles={[p]} />} />
                <Route path="/settings" element={<div>Settings page</div>} />
                <Route path="/setup" element={<div>Setup page</div>} />
            </Routes>
        </MemoryRouter>,
    );
}

beforeAll(() => {
    globalThis.ResizeObserver ??= class {
        observe() {}
        unobserve() {}
        disconnect() {}
    } as unknown as typeof ResizeObserver;
});

beforeEach(() => {
    vi.clearAllMocks();
    statsGet.mockResolvedValue({ data: createStatsResponse({ points: 48 }) });
    topGet.mockImplementation(async (_id: string, kind: string) => ({ data: kind === 'blocked' ? topBlocked : topResolved }));
    clientsGet.mockResolvedValue({ data: topClients });
    devicesGet.mockResolvedValue({ data: devicesList });
});

const headings = () => screen.getAllByRole('heading', { level: 2 }).map(h => h.textContent);

describe('Statistics page configurations', () => {
    it('OFF: one hero replaces everything below the description', async () => {
        // tableRef: statistics-behaviour #P1, #D2, #U1
        mount(mk('OFF'));
        expect(await screen.findByRole('heading', { name: 'Statistics are off' })).toBeInTheDocument();
        expect(screen.getByText('Nothing is stored for this profile.')).toBeInTheDocument();
        expect(screen.queryByRole('radiogroup', { name: 'Time range' })).not.toBeInTheDocument();
        expect(screen.queryByText('Counts')).not.toBeInTheDocument();
        expect(screen.getByRole('radio', { name: 'Statistics' })).toBeChecked();
        expect(screen.getByRole('button', { name: 'Turn on statistics' })).toBeEnabled();
        expect(statsGet).not.toHaveBeenCalled();
        await userEvent.setup().click(screen.getByRole('radio', { name: 'Query logs' }));
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toBeChecked();
        expect(screen.queryByRole('radiogroup', { name: 'Log domains' })).not.toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'More options in Settings' })).toBeInTheDocument();
    });

    it('S: counts with data, one G-LOGS block for the whole logs group', async () => {
        // tableRef: statistics-behaviour #P2, #U2
        mount(mk('S'));
        expect(await screen.findByLabelText(/^Total queries:/)).toBeInTheDocument();
        expect(headings()).toEqual(['Counts', 'From query logs']);
        expect(screen.getByRole('heading', { name: 'Domains and clients come from query logs' })).toBeInTheDocument();
        expect(screen.getByText('Query logs are off for this profile.')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Turn on query logs' })).toBeEnabled();
        expect(screen.queryByRole('heading', { name: 'Top blocked domains' })).not.toBeInTheDocument();
        expect(topGet).not.toHaveBeenCalled();
        expect(clientsGet).not.toHaveBeenCalled();
        expect(devicesGet).not.toHaveBeenCalled();
    });

    it('SL: everything with data and last seen in logs on devices', async () => {
        // tableRef: statistics-behaviour #P3
        mount(mk('SL'));
        expect(await screen.findByRole('heading', { name: 'Top blocked domains' })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: 'Top resolved domains' })).toBeInTheDocument();
        expect(await screen.findAllByText(/last seen in logs/)).not.toHaveLength(0);
        expect(devicesGet).toHaveBeenCalledTimes(1);
    });

    it('SL without client IPs: the clients panel is one G-IP block', async () => {
        // tableRef: statistics-behaviour #P8
        mount(mk('SL-ip'));
        expect(await screen.findByRole('heading', { name: "Client IPs aren't logged" })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Log client IPs' })).toBeEnabled();
        expect(clientsGet).not.toHaveBeenCalled();
    });

    it('SL with client IPs: the top clients table renders', async () => {
        mount(mk('SL'));
        expect(await screen.findByRole('heading', { name: 'Top clients' })).toBeInTheDocument();
        expect(clientsGet).toHaveBeenCalled();
    });

    it('SL without domains: top-domain panels are one G-DOM block', async () => {
        // tableRef: statistics-behaviour #P7
        mount(mk('SL-dom'));
        expect(await screen.findByRole('heading', { name: "Domains aren't logged" })).toBeInTheDocument();
        expect(screen.getByText('Query logs are on, but domain names are not kept.')).toBeInTheDocument();
        expect(screen.queryByRole('heading', { name: 'Top blocked domains' })).not.toBeInTheDocument();
        expect(topGet).not.toHaveBeenCalled();
    });

    it('L: the Counts group is one G-STATS block, logs panels render, no availability line', async () => {
        // tableRef: statistics-behaviour #P4, #U6
        mount(mk('L'));
        expect(await screen.findByRole('heading', { name: 'Counts are off' })).toBeInTheDocument();
        expect(screen.getByText('Query logs are on without statistics, so nothing is counted.')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Also keep statistics' })).toBeEnabled();
        expect(await screen.findByRole('heading', { name: 'Top blocked domains' })).toBeInTheDocument();
        expect(screen.queryByTestId('stats-availability')).not.toBeInTheDocument();
        expect(statsGet).not.toHaveBeenCalled();
    });
});

describe('Statistics page data states', () => {
    it('shows the availability line with the retention', async () => {
        // tableRef: statistics-behaviour #P13
        mount(mk('S'));
        expect(await screen.findByTestId('stats-availability')).toHaveTextContent(/Statistics on since .* · kept for 30 days/);
    });

    it('omits the date when enabled_at is null', async () => {
        // tableRef: statistics-behaviour #P13
        statsGet.mockResolvedValue({ data: createStatsResponse({ enabledAt: null }) });
        mount(mk('S'));
        expect(await screen.findByTestId('stats-availability')).toHaveTextContent('Statistics on · kept for 30 days');
    });

    it('no queries yet: one card with the 15 minute note and the setup link', async () => {
        // tableRef: statistics-behaviour #P5, #U2
        const user = userEvent.setup();
        statsGet.mockResolvedValue({ data: createStatsResponse({ empty: true, points: 4, enabledAt: '2026-10-06T14:50:00Z' }) });
        mount(mk('S'));
        expect(await screen.findByRole('heading', { name: 'No queries counted yet' })).toBeInTheDocument();
        expect(screen.getByText(/Counts appear about 15 minutes after the first query\./)).toBeInTheDocument();
        expect(screen.queryByLabelText(/^Total queries:/)).not.toBeInTheDocument();
        await user.click(screen.getByRole('link', { name: 'Check your device setup' }));
        expect(screen.getByText('Setup page')).toBeInTheDocument();
    });

    it('empty range: one card and a button for the next longer view', async () => {
        // tableRef: statistics-behaviour #P6
        const user = userEvent.setup();
        statsGet.mockResolvedValue({ data: createStatsResponse({ empty: true, points: 4 }) });
        mount(mk('S'), '/statistics?range=24h');
        expect(await screen.findByRole('heading', { name: 'No queries in the last 24 hours.' })).toBeInTheDocument();
        await user.click(screen.getByRole('button', { name: 'Show last 7 days' }));
        expect(screen.getByTestId('loc')).toHaveTextContent('/statistics');
        expect(screen.getByRole('radio', { name: 'Last 7 days' })).toBeChecked();
    });

    it('zero blocked: cards show zeros and the reasons panel says so', async () => {
        // tableRef: statistics-behaviour #P12
        statsGet.mockResolvedValue({ data: createStatsResponse({ blocked: false }) });
        mount(mk('S'));
        expect(await screen.findByText('Nothing was blocked in this range.')).toBeInTheDocument();
        expect(screen.getByLabelText('Blocked %: 0.0 %')).toBeInTheDocument();
    });

    it('error: one inline card with Try again, no toast', async () => {
        // tableRef: statistics-behaviour #P15
        const user = userEvent.setup();
        statsGet.mockRejectedValueOnce({ response: { status: 500 } });
        mount(mk('S'));
        expect(await screen.findByText("Couldn't load statistics")).toBeInTheDocument();
        expect(screen.getAllByRole('button', { name: 'Try again' })).toHaveLength(1);
        await user.click(screen.getByRole('button', { name: 'Try again' }));
        expect(await screen.findByLabelText(/^Total queries:/)).toBeInTheDocument();
        const { toast } = await import('sonner');
        expect(toast.error).not.toHaveBeenCalled();
    });

    it('429 reads as a rate limit message', async () => {
        // tableRef: statistics-behaviour #P15
        statsGet.mockRejectedValueOnce({ response: { status: 429 } });
        mount(mk('S'));
        expect(await screen.findByText('Too many requests. Try again in a minute.')).toBeInTheDocument();
    });

    it('a failing logs panel does not take down the counts', async () => {
        // tableRef: statistics-behaviour #P15
        topGet.mockRejectedValue({ response: { status: 500 } });
        mount(mk('SL'));
        expect(await screen.findByLabelText(/^Total queries:/)).toBeInTheDocument();
        expect(await screen.findAllByRole('button', { name: 'Try again' })).toHaveLength(2);
    });

    it('shows skeletons on first load', async () => {
        // tableRef: statistics-behaviour #P14
        let resolve!: (v: unknown) => void;
        statsGet.mockReturnValue(new Promise(r => (resolve = r)));
        mount(mk('S'));
        expect(screen.getByTestId('stats-counts-skeleton')).toBeInTheDocument();
        await act(async () => resolve({ data: createStatsResponse() }));
        expect(await screen.findByLabelText(/^Total queries:/)).toBeInTheDocument();
    });

    it('keeps the previous data at reduced opacity while a new range loads', async () => {
        // tableRef: statistics-behaviour #P14
        const user = userEvent.setup();
        mount(mk('S'));
        await screen.findByLabelText(/^Total queries:/);
        let resolve!: (v: unknown) => void;
        statsGet.mockReturnValue(new Promise(r => (resolve = r)));
        await user.click(screen.getByRole('radio', { name: 'Last 24 hours' }));
        const group = await screen.findByTestId('stats-counts-data');
        expect(group).toHaveAttribute('aria-busy', 'true');
        expect(group).toHaveClass('opacity-60');
        await act(async () => resolve({ data: createStatsResponse({ points: 24 }) }));
        await waitFor(() => expect(screen.getByTestId('stats-counts-data')).not.toHaveAttribute('aria-busy'));
    });

    it('drops the data and shows skeletons when the profile switches', async () => {
        // tableRef: statistics-behaviour #P14
        mount(mk('S', 'p1'));
        await screen.findByLabelText(/^Total queries:/);
        statsGet.mockReturnValue(new Promise(() => undefined));
        act(() => {
            const p2 = mk('S', 'p2');
            useAppStore.setState({ activeProfile: p2, profiles: [p2] });
        });
        expect(await screen.findByTestId('stats-counts-skeleton')).toBeInTheDocument();
    });

    it('clamp caption replaces the range caption when retention shortens the view', async () => {
        // tableRef: statistics-behaviour #P11
        statsGet.mockResolvedValue({ data: createStatsResponse({ points: 30, bucketSeconds: 86400, timespan: 'LAST_3_MONTHS' }) });
        mount(mk('S'), '/statistics?range=3m');
        expect(await screen.findByTestId('stats-range-caption')).toHaveTextContent(/^Statistics are kept for 30 days, so this view shows /);
    });

    it('captions the logs window when log retention is shorter than the view', async () => {
        // tableRef: statistics-behaviour #P18
        mount(mk('SL', 'p1', '1h'));
        expect(await screen.findByText('From the last 1 hour of query logs')).toBeInTheDocument();
    });

    it('store says on but the API says off: renders OFF and revalidates the profile once', async () => {
        // tableRef: statistics-behaviour #P17
        statsGet.mockResolvedValue({ data: { enabled: false, series: [], devices: [] } });
        profileGet.mockResolvedValue({ data: mk('OFF') });
        mount(mk('S'));
        expect(await screen.findByRole('heading', { name: 'Statistics are off' })).toBeInTheDocument();
        await waitFor(() => expect(profileGet).toHaveBeenCalledTimes(1));
        expect(screen.queryByLabelText(/^Total queries:/)).not.toBeInTheDocument();
    });
});

describe('Statistics page picker and fetching', () => {
    it('writes the range to the URL, omitting the default', async () => {
        // tableRef: statistics-behaviour #K2
        const user = userEvent.setup();
        mount(mk('S'));
        await screen.findByLabelText(/^Total queries:/);
        expect(screen.getByTestId('loc')).toHaveTextContent('/statistics');
        expect(screen.getByTestId('loc')).not.toHaveTextContent('range');
        await user.click(screen.getByRole('radio', { name: 'Last 30 days' }));
        expect(screen.getByTestId('loc')).toHaveTextContent('/statistics?range=30d');
        await user.click(screen.getByRole('radio', { name: 'Last 7 days' }));
        expect(screen.getByTestId('loc')).not.toHaveTextContent('range');
    });

    it('reads an invalid range as 7d', async () => {
        // tableRef: statistics-behaviour #K2
        mount(mk('S'), '/statistics?range=bogus');
        expect(await screen.findByRole('radio', { name: 'Last 7 days' })).toBeChecked();
        expect(statsGet).toHaveBeenCalledWith('p1', 'LAST_7_DAYS');
    });

    it('exposes seven named segments in a radiogroup', async () => {
        // tableRef: statistics-behaviour #K1
        mount(mk('S'));
        const group = await screen.findByRole('radiogroup', { name: 'Time range' });
        expect(within(group).getAllByRole('radio')).toHaveLength(7);
    });

    it('fetches statistics on a range change with the mapped timespans', async () => {
        // tableRef: statistics-behaviour #K3, #K4, #K7
        const user = userEvent.setup();
        mount(mk('SL'));
        await screen.findByRole('heading', { name: 'Top clients' });
        await user.click(screen.getByRole('radio', { name: 'Last 3 hours' }));
        await waitFor(() => expect(statsGet).toHaveBeenLastCalledWith('p1', 'LAST_3_HOURS'));
        await waitFor(() => expect(topGet).toHaveBeenCalledWith('p1', 'blocked', 'LAST_3_HOURS', 10));
        expect(clientsGet).toHaveBeenLastCalledWith('p1', 'LAST_3_HOURS', 10);
        // The device list does not depend on the range.
        expect(devicesGet).toHaveBeenCalledTimes(1);
    });

    it('ignores an answer for a range that is no longer current', async () => {
        // tableRef: statistics-behaviour #K8
        const user = userEvent.setup();
        let slow!: (v: unknown) => void;
        statsGet.mockImplementationOnce(() => new Promise(r => (slow = r)));
        mount(mk('S'));
        await user.click(await screen.findByRole('radio', { name: 'Last 24 hours' }));
        await screen.findByLabelText(/^Total queries:/);
        await act(async () => slow({ data: createStatsResponse({ points: 5 }) }));
        const total = createStatsResponse({ points: 48 }).totals.total;
        expect(screen.getByLabelText(/^Total queries:/)).toHaveAttribute('title', total.toLocaleString());
    });

    it('does not refetch on its own', async () => {
        // tableRef: statistics-behaviour #K9
        mount(mk('S'));
        await screen.findByLabelText(/^Total queries:/);
        expect(statsGet).toHaveBeenCalledTimes(1);
    });
});

describe('Statistics page gate actions and limited access', () => {
    it('G-LOGS opens the control in a dialog with Query logs focused, without a PATCH', async () => {
        // tableRef: statistics-behaviour #D4
        const user = userEvent.setup();
        mount(mk('S'));
        await user.click(await screen.findByRole('button', { name: 'Turn on query logs' }));
        const dialog = await screen.findByRole('dialog', { name: 'Data collection' });
        await waitFor(() => expect(within(dialog).getByRole('radio', { name: 'Query logs' })).toHaveFocus());
        expect(api.Client.profilesApi.apiV1ProfilesIdPatch).not.toHaveBeenCalled();
    });

    it('G-STATS focuses "Also keep statistics"', async () => {
        // tableRef: statistics-behaviour #D4
        const user = userEvent.setup();
        mount(mk('L'));
        await user.click(await screen.findByRole('button', { name: 'Also keep statistics' }));
        const dialog = await screen.findByRole('dialog', { name: 'Data collection' });
        await waitFor(() => expect(within(dialog).getByRole('checkbox', { name: 'Also keep statistics' })).toHaveFocus());
    });

    it('G-DOM and G-IP focus the matching sub-option', async () => {
        // tableRef: statistics-behaviour #D4
        const user = userEvent.setup();
        mount(mk('SL-dom'));
        await user.click(await screen.findByRole('button', { name: 'Log domains' }));
        const dialog = await screen.findByRole('dialog', { name: 'Data collection' });
        await waitFor(() => expect(within(dialog).getByRole('radiogroup', { name: 'Log domains' }).querySelector('input')).toHaveFocus());
    });

    it('under limited access data renders, gate actions are disabled and the reason is visible text', async () => {
        // tableRef: statistics-behaviour #P16, #C18
        mount(mk('S'));
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        expect(await screen.findByLabelText(/^Total queries:/)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Turn on query logs' })).toBeDisabled();
        for (const note of screen.getAllByText("Data collection can't be changed in limited access mode. You can still clear query logs.")) {
            expect(note).toBeVisible();
        }
    });

    it('under limited access the hero control is disabled with the reason visible', async () => {
        // tableRef: statistics-behaviour #P16
        mount(mk('OFF'));
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        await screen.findByRole('heading', { name: 'Statistics are off' });
        expect(screen.getByRole('radio', { name: 'Statistics' })).toBeDisabled();
        expect(screen.getByText(/can't be changed in limited access mode/)).toBeVisible();
    });
});

describe('Statistics page accessibility structure', () => {
    it('uses h2 group headings and h3 panel headings inside labelled sections', async () => {
        // tableRef: statistics-behaviour #X1
        mount(mk('SL'));
        await screen.findByRole('heading', { name: 'Top clients' });
        expect(headings()).toEqual(['Counts', 'From query logs']);
        for (const name of ['Queries over time', 'Blocked by reason', 'Protocols', 'Devices', 'Top blocked domains', 'Top resolved domains', 'Top clients']) {
            const h = screen.getByRole('heading', { name, level: 3 });
            expect(h.closest('section')).toHaveAttribute('aria-labelledby', h.id);
        }
    });
});

describe('Statistics page retention', () => {
    it('shows the Kept for control and menu whenever statistics are on', async () => {
        // tableRef: statistics-behaviour #S1
        mount(mk('S'));
        expect(await screen.findByLabelText('Kept for')).toHaveValue('30d');
        expect(screen.getByRole('button', { name: 'More statistics actions' })).toBeInTheDocument();
    });

    it('hides it when statistics are off', async () => {
        // tableRef: statistics-behaviour #S1
        mount(mk('L'));
        await screen.findByRole('heading', { name: 'Counts are off' });
        expect(screen.queryByLabelText('Kept for')).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'More statistics actions' })).not.toBeInTheDocument();
    });

    it('refetches statistics after the retention changes', async () => {
        // tableRef: statistics-behaviour #S1
        const user = userEvent.setup();
        const updated = mk('S');
        (updated.settings.statistics as unknown as { retention: string }).retention = '1y';
        (api.Client.profilesApi.apiV1ProfilesIdPatch as unknown as Mock).mockResolvedValue({ status: 200, data: updated });
        profileGet.mockResolvedValue({ data: mk('S') });
        mount(mk('S'));
        await user.selectOptions(await screen.findByLabelText('Kept for'), '1y');
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Keep for 1 year' }));
        await waitFor(() => expect(statsGet).toHaveBeenCalledTimes(2));
    });

    it('counts since the later of enabled_at and history_deleted_at', async () => {
        // tableRef: statistics-behaviour #P13
        statsGet.mockResolvedValue({ data: { ...createStatsResponse(), enabled_at: '2026-09-14T09:12:00Z', history_deleted_at: '2026-10-05T10:00:00Z' } });
        mount(mk('S'));
        const line = await screen.findByTestId('stats-availability');
        const fmt = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        expect(line).toHaveTextContent(`Statistics on since ${fmt.format(Date.parse('2026-10-05T10:00:00Z'))}`);
    });

    it('after Delete history shows the no-queries-yet card and moves focus to its heading', async () => {
        // tableRef: statistics-behaviour #S3, #P5
        const user = userEvent.setup();
        mount(mk('S'));
        await screen.findByLabelText(/^Total queries:/);
        statsDelete.mockResolvedValue({ status: 204 });
        profileGet.mockResolvedValue({ data: mk('S') });
        statsGet.mockResolvedValue({
            data: { ...createStatsResponse({ empty: true, points: 4, enabledAt: '2026-09-14T09:12:00Z' }), history_deleted_at: '2026-10-06T15:10:00Z' },
        });
        await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
        await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Delete history' }));
        const heading = await screen.findByRole('heading', { name: 'No queries counted yet' });
        expect(statsDelete).toHaveBeenCalledWith('p1');
        await waitFor(() => expect(heading).toHaveFocus());
    });
});
