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
    // Like the page, read the profile from the store so store writes reach the control.
    const Harness = () => {
        const current = useAppStore(st => st.activeProfile) ?? p;
        return <RetentionControl profile={current} onHistoryDeleted={onHistoryDeleted} />;
    };
    render(<Harness />);
    return { onHistoryDeleted };
}

const select = () => screen.getByLabelText('Kept for') as HTMLSelectElement;

beforeEach(() => {
    vi.clearAllMocks();
    get.mockImplementation(async (id: string) => ({ status: 200, data: useAppStore.getState().profiles.find(p => p.profile_id === id) }));
});

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
        expect(within(dialog).getByText("modDNS will keep this profile's query counts for up to 1 year. Query logs are not affected. You can lower this or delete the history at any time.")).toBeInTheDocument();
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

describe('RetentionControl stale check', () => {
    const C20 = 'This setting was changed elsewhere. Review it and try again.';

    it('applies nothing when the stored retention changed elsewhere and shows the server value', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        const server = mk('1y');
        get.mockResolvedValue({ status: 200, data: server });
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        await waitFor(() => expect(screen.getByText(C20)).toBeInTheDocument());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(patch).not.toHaveBeenCalled();
        expect(useAppStore.getState().activeProfile).toBe(server);
        await waitFor(() => expect(select().value).toBe('1y'));
        await waitFor(() => expect(select()).toHaveFocus());
        expect(screen.getByText(C20).closest('p')).toHaveAttribute('aria-live', 'polite');
    });

    it('clears the message on the next interaction', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        get.mockResolvedValueOnce({ status: 200, data: mk('1y') });
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        await screen.findByText(C20);
        get.mockImplementation(async () => ({ status: 200, data: mk('1y') }));
        await user.selectOptions(select(), '30d');
        await waitFor(() => expect(screen.queryByText(C20)).not.toBeInTheDocument());
    });

    it('treats a changed statistics.enabled as stale too', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        const off = mk('30d');
        (off.settings.statistics as unknown as { enabled: boolean }).enabled = false;
        get.mockResolvedValue({ status: 200, data: off });
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        await waitFor(() => expect(useAppStore.getState().activeProfile).toBe(off));
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(patch).not.toHaveBeenCalled();
    });

    it('opens the normal dialog when the re-read matches, and stays busy during the check', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        let resolve!: (v: unknown) => void;
        get.mockReturnValue(new Promise(r => (resolve = r)));
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        expect(select()).toBeDisabled();
        expect(select().value).toBe('90d');
        resolve({ status: 200, data: mk('30d') });
        expect(await screen.findByRole('dialog', { name: 'Keep statistics for 90 days?' })).toBeInTheDocument();
        expect(screen.queryByText(C20)).not.toBeInTheDocument();
    });

    it('proceeds when the re-read fails', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        get.mockRejectedValue(new Error('network'));
        mount(mk('30d'));
        await user.selectOptions(select(), '90d');
        expect(await screen.findByRole('dialog', { name: 'Keep statistics for 90 days?' })).toBeInTheDocument();
        expect(toast.error).not.toHaveBeenCalled();
    });

    it('does not delete the history when the profile changed elsewhere', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        get.mockResolvedValue({ status: 200, data: mk('1y') });
        mount(mk('30d'));
        await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
        await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
        await screen.findByText(C20);
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(del).not.toHaveBeenCalled();
    });

    it('opens the delete confirmation when the re-read matches', async () => {
        // tableRef: statistics-behaviour #S6
        const user = userEvent.setup();
        mount(mk('30d'));
        await user.click(screen.getByRole('button', { name: 'More statistics actions' }));
        await user.click(await screen.findByRole('menuitem', { name: 'Delete statistics history' }));
        expect(await screen.findByRole('dialog', { name: 'Delete statistics history?' })).toBeInTheDocument();
    });
});
