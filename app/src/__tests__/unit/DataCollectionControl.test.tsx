// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
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

const radio = (name: RegExp | string) => screen.getByRole('radio', { name });
const saveBtn = () => screen.getByRole('button', { name: /^(Save|Saving|Turn on|Turn off|Save changes)/ });
const discardBtn = () => screen.getByRole('button', { name: 'Discard' });

beforeEach(() => {
    vi.clearAllMocks();
    get.mockImplementation(async (id: string) => ({ status: 200, data: useAppStore.getState().profiles.find(p => p.profile_id === id) }));
});

describe('DataCollectionControl', () => {
    it('shows the saved level and an idle action row with disabled Save and Discard', () => {
        // tableRef: statistics-behaviour #T21
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(radio('Statistics')).toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
        expect(saveBtn()).toBeDisabled();
        expect(saveBtn()).toHaveTextContent('Save');
        expect(discardBtn()).toBeDisabled();
    });

    it('renders the level descriptions with the live logs retention', () => {
        // tableRef: statistics-behaviour #C2..C4
        const p = mk('p1', 'off', { retention: '1w' });
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText("Nothing is stored about this profile's queries.")).toBeInTheDocument();
        expect(screen.getByText('Query counts per device, kept for 30 days. No domains or IP addresses.')).toBeInTheDocument();
        expect(screen.getByText(/kept for 1 week\. Client IP addresses only if you turn them on\./)).toBeInTheDocument();
    });

    it('stages a level change without sending anything and shows the note and label', async () => {
        // tableRef: statistics-behaviour #T20, #T21, #C7
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(radio('Statistics'));
        expect(patch).not.toHaveBeenCalled();
        expect(screen.getByText(/modDNS will start counting this profile's queries per device and keep the counts for 30 days/)).toBeInTheDocument();
        expect(saveBtn()).toHaveTextContent('Turn on statistics');
        expect(saveBtn()).toBeEnabled();
        expect(discardBtn()).toBeEnabled();
    });

    it('never writes while arrow keys move through the radio group', async () => {
        // tableRef: statistics-behaviour #T20
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        radio('Off').focus();
        await user.keyboard('{ArrowDown}{ArrowDown}');
        expect(patch).not.toHaveBeenCalled();
    });

    it('Discard resets the staged state', async () => {
        // tableRef: statistics-behaviour #T21
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(radio('Statistics'));
        await user.click(discardBtn());
        expect(radio('Off')).toBeChecked();
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
        await user.click(radio('Statistics'));
        await user.click(saveBtn());

        expect(patch).toHaveBeenCalledTimes(1);
        expect(patch).toHaveBeenCalledWith('p1', {
            updates: [
                { operation: 'replace', path: '/settings/statistics/enabled', value: true },
                { operation: 'replace', path: '/settings/logs/enabled', value: false },
            ],
        });
        // No optimistic UI: busy, controls disabled, saved level not yet changed.
        expect(saveBtn()).toHaveTextContent('Saving…');
        expect(screen.getByRole('radiogroup')).toHaveAttribute('aria-busy', 'true');
        expect(radio('Off')).toBeDisabled();
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
        await user.click(radio('Off'));
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog', { name: 'Turn off statistics?' });
        expect(within(dialog).getByText(/All statistics for this profile will be permanently deleted\. This action cannot be undone\./)).toBeInTheDocument();
        expect(patch).not.toHaveBeenCalled();
        expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
    });

    it('Cancel closes the dialog, resets to saved and focuses the checked radio', async () => {
        // tableRef: statistics-behaviour #T22
        const user = userEvent.setup();
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(radio('Off'));
        await user.click(saveBtn());
        const dialog = await screen.findByRole('dialog');
        await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(patch).not.toHaveBeenCalled();
        expect(radio('Statistics')).toBeChecked();
        await waitFor(() => expect(radio('Statistics')).toHaveFocus());
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
        await user.click(radio('Off'));
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
        await user.click(radio('Statistics'));
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
        await user.click(radio('Statistics'));
        rerender(<DataCollectionControl profile={b} />);
        expect(radio('Off')).toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('under Query logs shows the checkbox, domains, IPs and retention (all placement)', () => {
        // tableRef: statistics-behaviour #D1, #C5, #C6
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toBeChecked();
        expect(screen.getByText('Counts per device for 30 days. No domains or addresses.')).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Log domains' })).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Log client IP addresses' })).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Retention period' })).toBeInTheDocument();
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
        expect(within(screen.getByRole('radiogroup', { name: 'Retention period' })).getAllByRole('radio').map(r => r.textContent))
            .toEqual(['1 H', '6 H', '1 D', '1 W', '1 M']);
        await user.click(within(domains).getByLabelText('Enable'));
        expect(within(domains).getByLabelText('Enable')).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('renders logs-only with the checkbox unchecked and does not repair it', () => {
        // tableRef: statistics-behaviour #L5
        const p = mk('p1', 'logsOnly');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(radio('Query logs')).toBeChecked();
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).not.toBeChecked();
        expect(screen.getByText('No unsaved changes.')).toBeInTheDocument();
    });

    it('unchecking "Also keep statistics" is a T9 deleting change', async () => {
        // tableRef: statistics-behaviour #T9, #C11
        const user = userEvent.setup();
        const p = mk('p1', 'logs');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        await user.click(screen.getByRole('checkbox', { name: 'Also keep statistics' }));
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
        await user.click(radio('Query logs'));
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toBeChecked();
        await user.click(within(screen.getByRole('radiogroup', { name: 'Retention period' })).getByLabelText('1 week'));
        expect(screen.getByText('modDNS will record each query for 1 week and keep counts per device for 30 days.')).toBeInTheDocument();
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
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toBeDisabled();
        expect(saveBtn()).toBeDisabled();
        expect(discardBtn()).toBeDisabled();
        expect(screen.queryByText('No unsaved changes.')).not.toBeInTheDocument();
    });

    it('Off hero placement: Statistics pre-selected, Save bar shows T1, only the checkbox as sub-option', async () => {
        // tableRef: statistics-behaviour #D2
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        seed(p);
        render(
            <DataCollectionControl
                profile={p}
                subOptions="keep"
                initialPending={{ level: 'stats', keep: true }}
                keepFooter={<a href="/settings">More options in Settings</a>}
            />,
        );
        expect(radio('Statistics')).toBeChecked();
        expect(saveBtn()).toHaveTextContent('Turn on statistics');
        await user.click(radio('Query logs'));
        expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toBeChecked();
        expect(screen.queryByRole('radiogroup', { name: 'Log domains' })).not.toBeInTheDocument();
        expect(screen.getByRole('link', { name: 'More options in Settings' })).toBeInTheDocument();
    });

    it('focuses the requested option on mount', async () => {
        // tableRef: statistics-behaviour #D4
        const p = mk('p1', 'stats');
        seed(p);
        render(<DataCollectionControl profile={p} initialPending={{ level: 'logs', keep: true }} initialFocus="keep" />);
        await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Also keep statistics' })).toHaveFocus());
    });
    it('re-reads the profile before saving and applies nothing when it changed elsewhere', async () => {
        // tableRef: statistics-behaviour #T27, #C20
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        const server = mk('p1', 'logs');
        seed(p);
        get.mockResolvedValue({ status: 200, data: server });
        render(<DataCollectionControl profile={p} />);
        await user.click(radio('Statistics'));
        await user.click(saveBtn());
        await waitFor(() => expect(useAppStore.getState().activeProfile).toBe(server));
        expect(get).toHaveBeenCalledWith('p1');
        expect(patch).not.toHaveBeenCalled();
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(screen.getByText('This setting was changed elsewhere. Review it and try again.')).toBeInTheDocument();
        expect(radio('Query logs')).toBeChecked();
        await waitFor(() => expect(radio('Query logs')).toHaveFocus());
        // Cleared on the next pending change.
        await user.click(radio('Off'));
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
        await user.click(radio('Statistics'));
        await user.click(saveBtn());
        expect(screen.getByRole('radiogroup')).toHaveAttribute('aria-busy', 'true');
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
        await user.click(radio('Off'));
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
        await user.click(radio('Statistics'));
        await user.click(saveBtn());
        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(toast.error).not.toHaveBeenCalled();
    });
    it('uses the profile\'s live statistics retention in the level descriptions and notes', async () => {
        // tableRef: statistics-behaviour #C3, #C5, #C7
        const user = userEvent.setup();
        const p = mk('p1', 'off');
        (p.settings.statistics as unknown as { retention: string }).retention = '1y';
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText('Query counts per device, kept for 1 year. No domains or IP addresses.')).toBeInTheDocument();
        await user.click(radio('Query logs'));
        expect(screen.getByText('Counts per device for 1 year. No domains or addresses.')).toBeInTheDocument();
        expect(screen.getByText(/keep counts per device for 1 year\./)).toBeInTheDocument();
    });

    it('treats a missing statistics retention as 30 days', () => {
        // tableRef: statistics-behaviour #C3
        const p = mk('p1', 'off');
        seed(p);
        render(<DataCollectionControl profile={p} />);
        expect(screen.getByText(/Query counts per device, kept for 30 days\./)).toBeInTheDocument();
    });

    it('Settings shows a read-only retention line under Statistics that links to the Statistics page', () => {
        // tableRef: statistics-behaviour #S4, #D1
        const p = mk('p1', 'stats');
        (p.settings.statistics as unknown as { retention: string }).retention = '90d';
        seed(p);
        render(
            <MemoryRouter>
                <DataCollectionControl profile={p} retentionLine />
            </MemoryRouter>,
        );
        expect(screen.getByText(/Counts kept for/)).toHaveTextContent('Counts kept for 90 days · Change on the Statistics page');
        expect(screen.getByRole('link', { name: 'Change on the Statistics page' })).toHaveAttribute('href', '/statistics');
    });

    it('Settings shows the line under "Also keep statistics" when it is saved and checked', () => {
        // tableRef: statistics-behaviour #S4, #D1
        const p = mk('p1', 'logs');
        seed(p);
        render(
            <MemoryRouter>
                <DataCollectionControl profile={p} retentionLine />
            </MemoryRouter>,
        );
        expect(screen.getByRole('link', { name: 'Change on the Statistics page' })).toBeInTheDocument();
    });

    it('does not show the line without the Settings flag or when statistics are off', () => {
        // tableRef: statistics-behaviour #S4
        const p = mk('p1', 'stats');
        seed(p);
        const { unmount } = render(<DataCollectionControl profile={p} />);
        expect(screen.queryByText(/Counts kept for/)).not.toBeInTheDocument();
        unmount();
        const off = mk('p1', 'off');
        render(
            <MemoryRouter>
                <DataCollectionControl profile={off} retentionLine />
            </MemoryRouter>,
        );
        expect(screen.queryByText(/Counts kept for/)).not.toBeInTheDocument();
    });
});
