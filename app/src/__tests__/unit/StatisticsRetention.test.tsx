// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import { RetentionControl } from '@/pages/statistics/RetentionControl';
import type { ModelProfile } from '@/api/client';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: { apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdGet: vi.fn() },
            statisticsApi: { apiV1ProfilesIdStatisticsDelete: vi.fn() },
        },
    },
}));

type Mock = ReturnType<typeof vi.fn>;
const patch = api.Client.profilesApi.apiV1ProfilesIdPatch as unknown as Mock;
const get = api.Client.profilesApi.apiV1ProfilesIdGet as unknown as Mock;
const del = api.Client.statisticsApi.apiV1ProfilesIdStatisticsDelete as unknown as Mock;

const mk = (retention?: string): ModelProfile =>
    ({
        id: 'p1',
        profile_id: 'p1',
        account_id: 'a',
        name: 'p1',
        settings: { logs: { enabled: false }, statistics: { enabled: true, retention } },
    }) as unknown as ModelProfile;

function mount(p: ModelProfile, onHistoryDeleted = vi.fn()) {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
    render(<RetentionControl profile={p} onHistoryDeleted={onHistoryDeleted} />);
    return { onHistoryDeleted };
}

const select = () => screen.getByLabelText('Kept for') as HTMLSelectElement;

beforeEach(() => vi.clearAllMocks());

describe('RetentionControl', () => {
    it('shows the profile retention and treats a missing one as 30 days', () => {
        // tableRef: statistics-behaviour #P13
        mount(mk(undefined));
        expect(select().value).toBe('30d');
        expect(within(select()).getAllByRole('option').map(o => o.textContent)).toEqual(['30 days', '90 days', '1 year']);
    });

    it('raising opens a confirmation, shows the target meanwhile, and saves one PATCH', async () => {
        // tableRef: statistics-behaviour #S1
        const user = userEvent.setup();
        const p = mk('30d');
        const updated = mk('1y');
        patch.mockResolvedValue({ status: 200, data: updated });
        mount(p);
        await user.selectOptions(select(), '1y');
        const dialog = await screen.findByRole('dialog', { name: 'Keep statistics for 1 year?' });
        expect(select().value).toBe('1y');
        expect(within(dialog).getByText(/modDNS will keep this profile's query counts per device for up to 1 year\. No domains or addresses are stored\./)).toBeInTheDocument();
        expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
        expect(patch).not.toHaveBeenCalled();
        await user.click(within(dialog).getByRole('button', { name: 'Keep for 1 year' }));
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [{ operation: 'replace', path: '/settings/statistics/retention', value: '1y' }],
        });
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Statistics are now kept for 1 year.'));
        expect(useAppStore.getState().activeProfile).toBe(updated);
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('lowering warns that older counts are deleted and uses a destructive confirm', async () => {
        // tableRef: statistics-behaviour #S2
        const user = userEvent.setup();
        patch.mockResolvedValue({ status: 200, data: mk('30d') });
        mount(mk('1y'));
        await user.selectOptions(select(), '30d');
        const dialog = await screen.findByRole('dialog', { name: 'Keep statistics for 30 days?' });
        expect(within(dialog).getByText('Counts older than 30 days will be permanently deleted. This action cannot be undone.')).toBeInTheDocument();
        await user.click(within(dialog).getByRole('button', { name: 'Delete older counts' }));
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Statistics are now kept for 30 days. Older counts deleted.'));
    });

    it('cancel restores the saved value and sends nothing', async () => {
        // tableRef: statistics-behaviour #S1
        const user = userEvent.setup();
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Cancel' }));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(select().value).toBe('30d');
        expect(patch).not.toHaveBeenCalled();
        await waitFor(() => expect(select()).toHaveFocus());
    });

    it('disables the control while saving', async () => {
        // tableRef: statistics-behaviour #S1
        const user = userEvent.setup();
        let resolve!: (v: unknown) => void;
        patch.mockReturnValue(new Promise(r => (resolve = r)));
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        const dialog = await screen.findByRole('dialog');
        await user.click(within(dialog).getByRole('button', { name: 'Keep for 90 days' }));
        expect(within(dialog).getByRole('button', { name: 'Saving…' })).toBeDisabled();
        expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeDisabled();
        resolve({ status: 200, data: mk('90d') });
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('refetches the profile and toasts on error, keeping the saved value', async () => {
        // tableRef: statistics-behaviour #S1
        const user = userEvent.setup();
        const p = mk('30d');
        patch.mockRejectedValue({ response: { data: { detail: 'boom' } } });
        get.mockResolvedValue({ status: 200, data: p });
        mount(p);
        await user.selectOptions(select(), '1y');
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Keep for 1 year' }));
        await waitFor(() =>
            expect(toast.error).toHaveBeenCalledWith("Couldn't change how long statistics are kept.", { description: 'boom' }),
        );
        expect(get).toHaveBeenCalledWith('p1');
        expect(select().value).toBe('30d');
    });

    it('deletes the history from the menu after a destructive confirmation', async () => {
        // tableRef: statistics-behaviour #S3
        const user = userEvent.setup();
        del.mockResolvedValue({ status: 204 });
        const fresh = mk('30d');
        get.mockResolvedValue({ status: 200, data: fresh });
        const { onHistoryDeleted } = mount(mk('30d'));
        await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
        await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
        const dialog = await screen.findByRole('dialog', { name: 'Delete statistics history?' });
        expect(within(dialog).getByText('All statistics for this profile will be permanently deleted. Statistics stay on and counting starts again now. This action cannot be undone.')).toBeInTheDocument();
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
        get.mockResolvedValue({ status: 200, data: mk('30d') });
        const { onHistoryDeleted } = mount(mk('30d'));
        await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
        await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
        await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Delete history' }));
        await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Couldn't delete the statistics history.", undefined));
        expect(onHistoryDeleted).not.toHaveBeenCalled();
    });

    it('is disabled under limited access with the reason visible', () => {
        // tableRef: statistics-behaviour #S5, #C18
        mount(mk('30d'));
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        return waitFor(() => {
            expect(select()).toBeDisabled();
            expect(screen.getByRole('button', { name: 'More statistics actions' })).toBeDisabled();
            expect(screen.getByText(/can't be changed in limited access mode/)).toBeVisible();
        });
    });
});
