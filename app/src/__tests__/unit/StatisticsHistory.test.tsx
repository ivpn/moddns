// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import { StatsActions } from '@/pages/statistics/StatsToolbar';
import type { ModelProfile } from '@/api/client';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: { apiV1ProfilesIdGet: vi.fn() },
            statisticsApi: { apiV1ProfilesIdStatisticsDelete: vi.fn() },
        },
    },
}));

type Mock = ReturnType<typeof vi.fn>;
const get = api.Client.profilesApi.apiV1ProfilesIdGet as unknown as Mock;
const del = api.Client.statisticsApi.apiV1ProfilesIdStatisticsDelete as unknown as Mock;

const C20 = 'This setting was changed elsewhere. Review it and try again.';

const mk = (enabled = true): ModelProfile =>
    ({
        id: 'p1',
        profile_id: 'p1',
        account_id: 'a',
        name: 'p1',
        settings: { logs: { enabled: false }, statistics: { enabled, retention: '30d' } },
    }) as unknown as ModelProfile;

function mount(p: ModelProfile, onHistoryDeleted = vi.fn()) {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
    render(<StatsActions profile={p} onHistoryDeleted={onHistoryDeleted} />);
    return { onHistoryDeleted };
}

async function openDelete(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
}

beforeEach(() => {
    vi.clearAllMocks();
    get.mockImplementation(async (id: string) => ({ status: 200, data: useAppStore.getState().profiles.find(p => p.profile_id === id) }));
});

describe('Delete statistics history', () => {
    it('deletes the history from the menu after a destructive confirmation', async () => {
        // tableRef: statistics-behaviour #S3
        const user = userEvent.setup();
        del.mockResolvedValue({ status: 204 });
        const { onHistoryDeleted } = mount(mk());
        const fresh = mk();
        get.mockResolvedValue({ status: 200, data: fresh });
        await openDelete(user);
        const dialog = await screen.findByRole('dialog', { name: 'Delete statistics history?' });
        expect(within(dialog).getByText('All statistics for this profile will be permanently deleted. Statistics stay on and counting starts again now. This action cannot be undone.')).toBeInTheDocument();
        expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
        expect(del).not.toHaveBeenCalled();
        await user.click(within(dialog).getByRole('button', { name: 'Delete history' }));
        await waitFor(() => expect(del).toHaveBeenCalledWith('p1'));
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Statistics history deleted.'));
        expect(onHistoryDeleted).toHaveBeenCalledTimes(1);
        expect(useAppStore.getState().activeProfile).toBe(fresh);
    });

    it('reports a failed deletion without calling onHistoryDeleted', async () => {
        // tableRef: statistics-behaviour #S3
        const user = userEvent.setup();
        del.mockRejectedValue({ response: { status: 500 } });
        const { onHistoryDeleted } = mount(mk());
        await openDelete(user);
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Delete history' }));
        await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Couldn't delete the statistics history.", undefined));
        expect(onHistoryDeleted).not.toHaveBeenCalled();
    });

    it('Cancel closes the dialog without deleting and returns focus to the menu button', async () => {
        // tableRef: statistics-behaviour #S3
        const user = userEvent.setup();
        mount(mk());
        await openDelete(user);
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(del).not.toHaveBeenCalled();
        await waitFor(() => expect(screen.getByRole('button', { name: 'More statistics actions' })).toHaveFocus());
    });

    it('is disabled under limited access with the reason visible', () => {
        // tableRef: statistics-behaviour #S5, #C18
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        const p = mk();
        useAppStore.setState({ activeProfile: p, profiles: [p] });
        render(<StatsActions profile={p} onHistoryDeleted={vi.fn()} />);
        expect(screen.getByRole('button', { name: 'More statistics actions' })).toBeDisabled();
        expect(screen.getByText(/can't be changed in limited access mode/)).toBeVisible();
    });
});

describe('Delete statistics history stale check', () => {
    it('does not delete when statistics were turned off elsewhere and shows C20', async () => {
        // tableRef: statistics-behaviour #S6, #C20
        const user = userEvent.setup();
        mount(mk());
        get.mockResolvedValue({ status: 200, data: mk(false) });
        await openDelete(user);
        await screen.findByText(C20);
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(del).not.toHaveBeenCalled();
        expect(useAppStore.getState().activeProfile?.settings?.statistics?.enabled).toBe(false);
    });

    it('opens the confirmation when the re-read matches', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        mount(mk());
        await openDelete(user);
        expect(await screen.findByRole('dialog', { name: 'Delete statistics history?' })).toBeInTheDocument();
    });

    it('proceeds when the re-read fails', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        mount(mk());
        get.mockRejectedValue(new Error('network'));
        await openDelete(user);
        expect(await screen.findByRole('dialog', { name: 'Delete statistics history?' })).toBeInTheDocument();
    });
});
