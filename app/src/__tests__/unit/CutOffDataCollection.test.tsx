import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ModelProfile } from '@/api/client';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import CutOffDataCollection from '@/components/data-collection/CutOffDataCollection';

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: { apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdGet: vi.fn() },
            queryLogsApi: { apiV1ProfilesIdLogsDelete: vi.fn() },
            statisticsApi: { apiV1ProfilesIdStatisticsDelete: vi.fn() },
        },
    },
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

type Mock = ReturnType<typeof vi.fn>;
const patch = api.Client.profilesApi.apiV1ProfilesIdPatch as unknown as Mock;
const get = api.Client.profilesApi.apiV1ProfilesIdGet as unknown as Mock;
const clearLogs = api.Client.queryLogsApi.apiV1ProfilesIdLogsDelete as unknown as Mock;

const CUT_OFF = "Your account can't change data collection now. You can still turn it off and delete what is stored.";

function mk(id: string, stats: boolean, logs: boolean): ModelProfile {
    return {
        id,
        profile_id: id,
        account_id: 'a',
        name: `Profile ${id}`,
        settings: {
            logs: { enabled: logs, log_domains: true, log_clients_ips: false, retention: '1h' },
            statistics: { enabled: stats, retention: '30d' },
        },
    } as unknown as ModelProfile;
}

function seed(profiles: ModelProfile[]) {
    useAppStore.setState({ profiles, activeProfile: profiles[0] ?? null, subscriptionStatus: 'inactive' });
    get.mockImplementation(async (id: string) => ({ status: 200, data: useAppStore.getState().profiles.find(p => p.profile_id === id) }));
}

beforeEach(() => vi.clearAllMocks());

describe('Data collection while the account is cut off', () => {
    it('lists only the profiles that collect something, once with the reason', () => {
        // tableRef: statistics-behaviour #D5, #C26
        seed([mk('a', true, true), mk('b', false, false), mk('c', false, true)]);
        render(<CutOffDataCollection />);
        expect(screen.getByRole('heading', { name: 'DATA COLLECTION' })).toBeInTheDocument();
        expect(screen.getAllByText(CUT_OFF)).toHaveLength(1);
        expect(screen.getByRole('heading', { name: 'Profile a' })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: 'Profile c' })).toBeInTheDocument();
        expect(screen.queryByRole('heading', { name: 'Profile b' })).not.toBeInTheDocument();
        const c = screen.getByRole('region', { name: 'Profile c' });
        expect(within(c).queryByRole('button', { name: 'Delete statistics history' })).not.toBeInTheDocument();
        expect(within(c).getByRole('button', { name: 'Clear query logs' })).toBeEnabled();
    });

    it('turns a profile off with only the false path, without switching the active profile', async () => {
        // tableRef: statistics-behaviour #D5, #L10
        const user = userEvent.setup();
        const [a, c] = [mk('a', true, true), mk('c', true, false)];
        seed([a, c]);
        patch.mockResolvedValue({ status: 200, data: mk('c', false, false) });
        render(<CutOffDataCollection />);
        const region = screen.getByRole('region', { name: 'Profile c' });
        await user.click(within(region).getByRole('checkbox', { name: 'Statistics' }));
        await user.click(within(region).getByRole('button', { name: 'Turn off' }));
        const dialog = await screen.findByRole('dialog', { name: 'Turn off statistics?' });
        await user.click(within(dialog).getByRole('button', { name: 'Turn off and delete' }));
        await waitFor(() => expect(patch).toHaveBeenCalledWith('c', {
            updates: [{ operation: 'replace', path: '/settings/statistics/enabled', value: false }],
        }));
        expect(useAppStore.getState().activeProfile?.profile_id).toBe('a');
        await waitFor(() => expect(screen.queryByRole('heading', { name: 'Profile c' })).not.toBeInTheDocument());
    });

    it('clears query logs behind a confirmation', async () => {
        // tableRef: statistics-behaviour #D5
        const user = userEvent.setup();
        seed([mk('a', false, true)]);
        clearLogs.mockResolvedValue({ status: 204 });
        render(<CutOffDataCollection />);
        await user.click(screen.getByRole('button', { name: 'Clear query logs' }));
        const dialog = await screen.findByRole('dialog', { name: 'Clear all query logs?' });
        await user.click(within(dialog).getByRole('button', { name: 'Clear logs' }));
        await waitFor(() => expect(clearLogs).toHaveBeenCalledWith('a'));
    });

    it('opens Delete statistics history for a profile with statistics on', async () => {
        // tableRef: statistics-behaviour #D5, #S3
        const user = userEvent.setup();
        seed([mk('a', true, false)]);
        render(<CutOffDataCollection />);
        await user.click(screen.getByRole('button', { name: 'Delete statistics history' }));
        expect(await screen.findByRole('dialog', { name: 'Delete statistics history?' })).toBeInTheDocument();
    });

    it('says so when nothing is collected', () => {
        // tableRef: statistics-behaviour #D5
        seed([mk('a', false, false)]);
        render(<CutOffDataCollection />);
        expect(screen.getByText("Nothing is stored about your profiles' queries.")).toBeInTheDocument();
        expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    });
});
