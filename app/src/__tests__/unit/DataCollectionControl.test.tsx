// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import api from '@/api/api';
import { useAppStore } from '@/store/general';
import { DataCollectionControl } from '@/components/data-collection/DataCollectionControl';
import type { ModelProfile } from '@/api/client';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: {
                apiV1ProfilesIdPatch: vi.fn(),
                apiV1ProfilesIdGet: vi.fn(),
            },
        },
    },
}));

type Mock = ReturnType<typeof vi.fn>;
const patch = api.Client.profilesApi.apiV1ProfilesIdPatch as unknown as Mock;
const get = api.Client.profilesApi.apiV1ProfilesIdGet as unknown as Mock;

function mk(id: string, level: 'off' | 'stats' | 'logs' | 'logsOnly', extra: Record<string, unknown> = {}): ModelProfile {
    return {
        id,
        profile_id: id,
        account_id: 'a',
        name: id,
        settings: {
            logs: {
                enabled: level === 'logs' || level === 'logsOnly',
                log_domains: true,
                log_clients_ips: false,
                retention: '1h',
                ...extra,
            },
            statistics: { enabled: level === 'stats' || level === 'logs' },
        },
    } as unknown as ModelProfile;
}

function seed(p: ModelProfile) {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
}

const box = (name: 'Statistics' | 'Query logs') => screen.getByRole('checkbox', { name });
const saveBtn = () => screen.getByRole('button', { name: /^(Save|Saving|Turn on|Turn off|Save changes|Keep for)/ });
const group = (name: string) => screen.getByRole('radiogroup', { name });
const discardBtn = () => screen.getByRole('button', { name: 'Discard' });

beforeEach(() => {
    vi.clearAllMocks();
    get.mockImplementation(async (id: string) => ({ status: 200, data: useAppStore.getState().profiles.find(p => p.profile_id === id) }));
});

describe('DataCollectionControl', () => {
    it('shows the saved sources and an idle action row with disabled Save and Discard', () => {
        // tableRef: statistics-behaviour #T21, #L2
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(box('Statistics')).toBeChecked();
        expect(box('Query logs')).not.toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
        expect(saveBtn()).toBeDisabled();
        expect(saveBtn()).toHaveTextContent('Save');
        expect(discardBtn()).toBeDisabled();
    });

    it('renders the card descriptions with the live logs retention and the Off status line', () => {
        // tableRef: statistics-behaviour #C2..C4, #L1, #L9
        const p = mk('p1', 'off', { retention: '1w' });
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByTestId('data-collection-status')).toHaveTextContent("Off · Nothing is stored about this profile's queries.");
        expect(screen.getByText('Query counts per device, kept for 30 days. No domains or IP addresses.')).toBeInTheDocument();
        expect(screen.getByText(/kept for 1 week\. Client IP addresses only if you turn them on\./)).toBeInTheDocument();
    });

    it('names the saved sources in the status line', () => {
        // tableRef: statistics-behaviour #L9, #C2
        const p = mk('p1', 'logs', { retention: '1d' });
        (p.settings.statistics as unknown as { retention: string }).retention = '90d';
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByTestId('data-collection-status')).toHaveTextContent('Collecting now:Statistics · 90 daysQuery logs · 1 day');
    });

    it('stages a source change without sending anything and shows the note and label', async () => {
        // tableRef: statistics-behaviour #T20, #T21, #C7
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        expect(patch).not.toHaveBeenCalled();
        expect(screen.getByText(/modDNS will start counting this profile's queries per device and keep the counts for 30 days/)).toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on statistics');
        expect(saveBtn()).toBeEnabled();
        expect(discardBtn()).toBeEnabled();
    });

    it('never writes while arrow keys move through a pill group', async () => {
        // tableRef: statistics-behaviour #T20
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        within(group('Statistics Retention period')).getByLabelText('30 days').focus();
        await user.keyboard('{ArrowRight}{ArrowRight}');
        expect(patch).not.toHaveBeenCalled();
    });

    it('Discard resets the staged state', async () => {
        // tableRef: statistics-behaviour #T21
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(discardBtn());
        expect(box('Statistics')).not.toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('saves a non-deleting change as one PATCH with both enabled paths, writes the returned profile and toasts', async () => {
        // tableRef: statistics-behaviour #T1, #T23, #T24, #C16
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        const updated = mk('p1', 'stats');
        seed(p);
        let resolve!: (v: unknown) => void;
        patch.mockReturnValue(new Promise(r => (resolve = r)));
        const onSaved = vi.fn();
        render(<DataCollectionControl profile={p} onSaved={onSaved} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());

        expect(patch).toHaveBeenCalledTimes(1);
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [
                { operation: 'replace', path: '/settings/statistics/enabled', value: true },
                { operation: 'replace', path: '/settings/logs/enabled', value: false },
            ],
        });
        // No optimistic UI: busy, controls disabled, saved state not yet changed.
        expect(saveBtn()).toHaveTextContent('Saving…');
        expect(screen.getByRole('group', { name: 'Data collection' })).toHaveAttribute('aria-busy', 'true');
        expect(box('Statistics')).toBeDisabled();
        expect(useAppStore.getState().activeProfile).toBe(p);

        resolve({ status: 200, data: updated });
        await waitFor(() => expect(useAppStore.getState().activeProfile).toBe(updated));
        expect(useAppStore.getState().profiles[0]).toBe(updated);
        expect(toast.success).toHaveBeenCalledWith('Statistics turned on.');
        expect(onSaved).toHaveBeenCalledWith(updated, expect.objectContaining({ id: 'T1' }));
    });

    it('opens a confirmation dialog for a deleting change with Cancel first and focused', async () => {
        // tableRef: statistics-behaviour #T4, #T22, #C8
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog', { name: 'Turn off statistics?' });
        expect(within(dialog).getByText(/All statistics for this profile will be permanently deleted\. This action cannot be undone\./)).toBeInTheDocument();
        expect(patch).not.toHaveBeenCalled();
        expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
    });

    it('Cancel closes the dialog, resets to saved and focuses the touched source', async () => {
        // tableRef: statistics-behaviour #T22
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Query logs'));
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog', { name: 'Turn off query logs?' });
        await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(patch).not.toHaveBeenCalled();
        expect(box('Query logs')).toBeChecked();
        await waitFor(() => expect(box('Query logs')).toHaveFocus());
    });

    it('confirming sends the PATCH and shows "Deleting…" while pending', async () => {
        // tableRef: statistics-behaviour #T7, #T23, #C9, #C16
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        const updated = mk('p1', 'off');
        seed(p);
        let resolve!: (v: unknown) => void;
        patch.mockReturnValue(new Promise(r => (resolve = r)));
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(box('Query logs'));
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog', { name: 'Turn off data collection?' });
        await user.click(within(dialog).getByRole('button', { name: 'Turn off and delete' }));
        expect(within(dialog).getByRole('button', { name: 'Deleting…' })).toBeDisabled();
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [
                { operation: 'replace', path: '/settings/statistics/enabled', value: false },
                { operation: 'replace', path: '/settings/logs/enabled', value: false },
            ],
        });
        resolve({ status: 200, data: updated });
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Data collection turned off. Logs and statistics deleted.'));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('refetches the profile and renders the server state after an error', async () => {
        // tableRef: statistics-behaviour #T25, #C17
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        const committed = mk('p1', 'stats');
        seed(p);
        patch.mockRejectedValue({ response: { data: { detail: 'Redis unavailable' } } });
        get.mockResolvedValueOnce({ status: 200, data: p }).mockResolvedValueOnce({ status: 200, data: committed });
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        await waitFor(() => expect(get).toHaveBeenCalledWith('p1'));
        expect(toast.error).toHaveBeenCalledWith(
            "Couldn't save the data collection setting. Showing the current setting.",
            { description: 'Redis unavailable' },
        );
        await waitFor(() => expect(useAppStore.getState().activeProfile).toBe(committed));
        expect(patch).toHaveBeenCalledTimes(1);
    });

    it('drops staged changes silently when the profile switches', async () => {
        // tableRef: statistics-behaviour #T26
        const user = userEvent.setup();
        const a = mk('a', 'off');
        const b = mk('b', 'off');
        seed(a);
        const { rerender } = render(<DataCollectionControl profile={a} />);
        await user.click(box('Statistics'));
        rerender(<DataCollectionControl profile={b} />);
        expect(box('Statistics')).not.toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('both cards show their sub-options when checked (all placement)', () => {
        // tableRef: statistics-behaviour #D1, #L6, #C5, #C6
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(box('Statistics')).toBeChecked();
        expect(box('Query logs')).toBeChecked();
        expect(screen.getByText('How long counts are kept. Charts can go back this far.')).toBeInTheDocument();
        expect(within(group('Statistics Retention period')).getAllByRole('radio').map(r => r.textContent)).toEqual(['30 D', '90 D', '1 Y']);
        expect(screen.getByRole('radiogroup', { name: 'Log domains' })).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Log client IP addresses' })).toBeInTheDocument();
        expect(group('Query logs Retention period')).toBeInTheDocument();
        expect(screen.getByTestId('retention-info-trigger')).toBeInTheDocument();
    });

    it('sub-option pills show Enable/Disable and 1 H to 1 M, and re-pressing the selected pill keeps it', async () => {
        // tableRef: statistics-behaviour #C6, #T20
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        const domains = screen.getByRole('radiogroup', { name: 'Log domains' });
        expect(within(domains).getAllByRole('radio').map(r => r.textContent)).toEqual(['Disable', 'Enable']);
        expect(within(group('Query logs Retention period')).getAllByRole('radio').map(r => r.textContent))
            .toEqual(['1 H', '6 H', '1 D', '1 W', '1 M']);
        await user.click(within(domains).getByLabelText('Enable'));
        expect(within(domains).getByLabelText('Enable')).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('renders logs-only with only Query logs checked and does not repair it', () => {
        // tableRef: statistics-behaviour #L5, #L4
        const p = mk('p1', 'logsOnly');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(box('Query logs')).toBeChecked();
        expect(box('Statistics')).not.toBeChecked();
        expect(screen.queryByRole('radiogroup', { name: 'Statistics Retention period' })).not.toBeInTheDocument();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('unchecking Statistics while logs stay on is a T9 deleting change', async () => {
        // tableRef: statistics-behaviour #T9, #C11
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        expect(await screen.findByRole('dialog', { name: 'Stop keeping statistics?' })).toBeInTheDocument();
    });

    it('a sub-option-only change saves without a dialog and sends only that path', async () => {
        // tableRef: statistics-behaviour #T13, #C16
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        const updated = mk('p1', 'logs', { log_clients_ips: true });
        seed(p);
        patch.mockResolvedValue({ status: 200, data: updated });
        render(<DataCollectionControl profile={p} />);
        const ips = screen.getByRole('radiogroup', { name: 'Log client IP addresses' });
        await user.click(within(ips).getByLabelText('Enable'));
        expect(saveBtn()).toHaveTextContent('Save changes');
        await user.click(saveBtn());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [{ operation: 'replace', path: '/settings/logs/log_clients_ips', value: true }],
        });
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Data collection updated.'));
    });

    it('shows the existing-rows hint when turning domains or IPs off, and nothing is deleted', async () => {
        // tableRef: statistics-behaviour #L7, #C15
        const user = userEvent.setup();
        const p = mk('p1', 'logs', { log_clients_ips: true });
        seed(p);
        render(<DataCollectionControl profile={p} />);
        const domains = screen.getByRole('radiogroup', { name: 'Log domains' });
        await user.click(within(domains).getByLabelText('Disable'));
        expect(screen.getByText('Applies to new queries. Existing logs keep their domains until they expire or you clear them.')).toBeInTheDocument();
        const ips = screen.getByRole('radiogroup', { name: 'Log client IP addresses' });
        await user.click(within(ips).getByLabelText('Disable'));
        expect(screen.getByText('Applies to new queries. Existing logs keep their IP addresses until they expire or you clear them.')).toBeInTheDocument();
        await user.click(saveBtn());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('stages sub-options with the level before logs first turn on', async () => {
        // tableRef: statistics-behaviour #L6, #T2
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        const updated = mk('p1', 'logs', { retention: '1w' });
        seed(p);
        patch.mockResolvedValue({ status: 200, data: updated });
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Query logs'));
        expect(box('Statistics')).toBeChecked();
        await user.click(within(group('Query logs Retention period')).getByLabelText('1 week'));
        expect(screen.getByText('modDNS will record each query for 1 week and keep counts per device for 30 days. The first counts appear at the next quarter hour.')).toBeInTheDocument();
        await user.click(saveBtn());
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [
                { operation: 'replace', path: '/settings/statistics/enabled', value: true },
                { operation: 'replace', path: '/settings/logs/enabled', value: true },
                { operation: 'replace', path: '/settings/logs/retention', value: '1w' },
            ],
        });
    });

    it('disables everything under limited access and shows the reason as visible text', async () => {
        // tableRef: statistics-behaviour #C18, #T21
        const p = mk('p1', 'logs');
        seed(p);
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText("Data collection can't be changed in limited access mode. You can still clear query logs.")).toBeVisible();
        for (const r of screen.getAllByRole('radio')) expect(r).toBeDisabled();
        expect(box('Statistics')).toBeDisabled();
        expect(box('Query logs')).toBeDisabled();
        expect(saveBtn()).toBeDisabled();
        expect(discardBtn()).toBeDisabled();
        expect(screen.queryByText('No unsaved changes.')).not.toBeInTheDocument();
    });

    it('Off hero placement: only the Statistics card, pre-checked with its retention, Save bar shows T1', () => {
        // tableRef: statistics-behaviour #D2, #T16
        const p = mk('p1', 'off');
        seed(p);
        render(
            <DataCollectionControl
                profile={p}
                sources="stats"
                initialPending={{ stats: true }}
                footer={<a href="/settings">More options in Settings</a>}
            />,
        );
        expect(box('Statistics')).toBeChecked();
        expect(screen.queryByRole('checkbox', { name: 'Query logs' })).not.toBeInTheDocument();
        expect(group('Statistics Retention period')).toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on statistics');
        expect(screen.getByRole('link', { name: 'More options in Settings' })).toBeInTheDocument();
    });

    it('focuses the requested source on mount', async () => {
        // tableRef: statistics-behaviour #D4
        const p = mk('p1', 'logsOnly');
        seed(p);
        render(<DataCollectionControl profile={p} initialPending={{ stats: true }} initialFocus="stats" />);
        await waitFor(() => expect(box('Statistics')).toHaveFocus());
    });

    it('the deep link focuses the checked statistics retention pill, or the checkbox while statistics are off', async () => {
        // tableRef: statistics-behaviour #S4
        const p = mk('p1', 'stats');
        (p.settings.statistics as unknown as { retention: string }).retention = '90d';
        seed(p);
        const { unmount } = render(<DataCollectionControl profile={p} initialFocus="stats-retention" />);
        await waitFor(() => expect(within(group('Statistics Retention period')).getByLabelText('90 days')).toHaveFocus());
        unmount();
        const off = mk('p1', 'off');
        seed(off);
        render(<DataCollectionControl profile={off} initialFocus="stats-retention" />);
        await waitFor(() => expect(box('Statistics')).toHaveFocus());
    });
    it('re-reads the profile before saving and applies nothing when it changed elsewhere', async () => {
        // tableRef: statistics-behaviour #T27, #C20
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        const server = mk('p1', 'logs');
        seed(p);
        get.mockResolvedValue({ status: 200, data: server });
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        await waitFor(() => expect(useAppStore.getState().activeProfile).toBe(server));
        expect(get).toHaveBeenCalledWith('p1');
        expect(patch).not.toHaveBeenCalled();
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(screen.getByText('This setting was changed elsewhere. Review it and try again.')).toBeInTheDocument();
        expect(box('Query logs')).toBeChecked();
        await waitFor(() => expect(box('Statistics')).toHaveFocus());
        // Cleared on the next pending change.
        await user.click(box('Query logs'));
        expect(screen.queryByText('This setting was changed elsewhere. Review it and try again.')).not.toBeInTheDocument();
    });

    it('keeps the control busy during the stale check', async () => {
        // tableRef: statistics-behaviour #T27
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        let resolve!: (v: unknown) => void;
        get.mockReturnValue(new Promise(r => (resolve = r)));
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        expect(screen.getByRole('group', { name: 'Data collection' })).toHaveAttribute('aria-busy', 'true');
        expect(saveBtn()).toBeDisabled();
        resolve({ status: 200, data: p });
        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
    });

    it('goes through the dialog only after a fresh re-read confirms the saved state', async () => {
        // tableRef: statistics-behaviour #T27
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        seed(p);
        get.mockResolvedValue({ status: 200, data: mk('p1', 'stats') });
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        expect(await screen.findByRole('dialog', { name: 'Turn off statistics?' })).toBeInTheDocument();
        expect(screen.queryByText('This setting was changed elsewhere. Review it and try again.')).not.toBeInTheDocument();
    });

    it('proceeds without an extra error when the re-read fails', async () => {
        // tableRef: statistics-behaviour #T27
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        get.mockRejectedValue(new Error('network'));
        patch.mockResolvedValue({ status: 200, data: mk('p1', 'stats') });
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Statistics'));
        await user.click(saveBtn());
        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(toast.error).not.toHaveBeenCalled();
    });
    it('uses the pending statistics retention in the card description and notes', async () => {
        // tableRef: statistics-behaviour #C3, #C7
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        (p.settings.statistics as unknown as { retention: string }).retention = '1y';
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText('Query counts per device, kept for 1 year. No domains or IP addresses.')).toBeInTheDocument();
        await user.click(box('Query logs'));
        expect(screen.getByText(/keep counts per device for 1 year\./)).toBeInTheDocument();
        await user.click(within(group('Statistics Retention period')).getByLabelText('90 days'));
        expect(screen.getByText('Query counts per device, kept for 90 days. No domains or IP addresses.')).toBeInTheDocument();
        expect(screen.getByText(/keep counts per device for 90 days\./)).toBeInTheDocument();
    });

    it('treats a missing statistics retention as 30 days', () => {
        // tableRef: statistics-behaviour #C3
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText(/Query counts per device, kept for 30 days\./)).toBeInTheDocument();
    });

    it('checking Query logs from Off also checks Statistics with a hint; unchecking it gives logs only', async () => {
        // tableRef: statistics-behaviour #L8, #C22, #T2, #T3
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Query logs'));
        expect(box('Statistics')).toBeChecked();
        expect(screen.getByText('Checked with query logs. Uncheck Statistics to keep query logs only.')).toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on statistics and query logs');
        await user.click(box('Statistics'));
        expect(screen.queryByText(/Checked with query logs/)).not.toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on query logs');
        expect(screen.getByText('modDNS will record each query for 1 hour. No counts are kept.')).toBeInTheDocument();
    });

    it('unchecking Query logs again returns Statistics to its saved state', async () => {
        // tableRef: statistics-behaviour #L8
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Query logs'));
        await user.click(box('Query logs'));
        expect(box('Statistics')).not.toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('checking Query logs with statistics already on changes only that card', async () => {
        // tableRef: statistics-behaviour #L8, #T5
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(box('Query logs'));
        expect(box('Statistics')).toBeChecked();
        expect(screen.queryByText(/Checked with query logs/)).not.toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on query logs');
    });

    it('starts with the L8 hint when a placement opens it with both checked from Off', () => {
        // tableRef: statistics-behaviour #D3, #L8
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} initialPending={{ logs: true, stats: true }} />);
        expect(screen.getByText('Checked with query logs. Uncheck Statistics to keep query logs only.')).toBeInTheDocument();
    });

    it('raising the statistics retention saves one PATCH without a dialog', async () => {
        // tableRef: statistics-behaviour #T14, #C23, #S1
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        const updated = mk('p1', 'stats');
        (updated.settings.statistics as unknown as { retention: string }).retention = '1y';
        seed(p);
        patch.mockResolvedValue({ status: 200, data: updated });
        render(<DataCollectionControl profile={p} />);
        await user.click(within(group('Statistics Retention period')).getByLabelText('1 year'));
        expect(screen.getByText("modDNS will keep this profile's counts for up to 1 year.")).toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Keep for 1 year');
        await user.click(saveBtn());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [{ operation: 'replace', path: '/settings/statistics/retention', value: '1y' }],
        });
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Statistics are now kept for 1 year.'));
    });

    it('lowering the statistics retention shows the hint and confirms before deleting older counts', async () => {
        // tableRef: statistics-behaviour #T15, #C24
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        (p.settings.statistics as unknown as { retention: string }).retention = '1y';
        seed(p);
        patch.mockResolvedValue({ status: 200, data: mk('p1', 'stats') });
        render(<DataCollectionControl profile={p} />);
        await user.click(within(group('Statistics Retention period')).getByLabelText('30 days'));
        expect(screen.getByText('Counts older than 30 days will be deleted when you save.')).toBeInTheDocument();
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog', { name: 'Keep statistics for 30 days?' });
        expect(patch).not.toHaveBeenCalled();
        await user.click(within(dialog).getByRole('button', { name: 'Delete older counts' }));
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [{ operation: 'replace', path: '/settings/statistics/retention', value: '30d' }],
        });
        await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Statistics are now kept for 30 days. Older counts deleted.'));
    });

    it('a retention change made elsewhere counts as stale', async () => {
        // tableRef: statistics-behaviour #T27
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        const server = mk('p1', 'stats');
        (server.settings.statistics as unknown as { retention: string }).retention = '90d';
        seed(p);
        get.mockResolvedValue({ status: 200, data: server });
        render(<DataCollectionControl profile={p} />);
        await user.click(within(group('Statistics Retention period')).getByLabelText('1 year'));
        await user.click(saveBtn());
        await waitFor(() => expect(screen.getByText('This setting was changed elsewhere. Review it and try again.')).toBeInTheDocument());
        expect(patch).not.toHaveBeenCalled();
    });
});
