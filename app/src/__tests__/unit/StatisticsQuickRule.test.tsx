// @vitest-environment jsdom
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import Statistics from '@/pages/statistics/Statistics';
import { resetQuickRuleSession } from '@/pages/statistics/groups';
import { useAppStore } from '@/store/general';
import api from '@/api/api';
import type { ModelProfile } from '@/api/client';
import { createStatsResponse, devicesList, topBlocked, topBlocklists, topClients, topResolved } from '../mocks/statisticsMocks';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() } }));
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            statisticsApi: { apiV1ProfilesIdStatisticsGet: vi.fn(), apiV1ProfilesIdStatisticsDelete: vi.fn() },
            queryLogsApi: {
                apiV1ProfilesIdLogsTopGet: vi.fn(),
                apiV1ProfilesIdLogsClientsGet: vi.fn(),
                apiV1ProfilesIdLogsBlocklistsGet: vi.fn(),
                apiV1ProfilesIdLogsDevicesGet: vi.fn(),
            },
            blocklistsApi: { apiV1BlocklistsGet: vi.fn() },
            profilesApi: { apiV1ProfilesIdGet: vi.fn(), apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdCustomRulesBatchPost: vi.fn() },
        },
    },
}));

type Mock = ReturnType<typeof vi.fn>;
const c = api.Client;
const statsGet = c.statisticsApi.apiV1ProfilesIdStatisticsGet as unknown as Mock;
const topGet = c.queryLogsApi.apiV1ProfilesIdLogsTopGet as unknown as Mock;
const clientsGet = c.queryLogsApi.apiV1ProfilesIdLogsClientsGet as unknown as Mock;
const blocklistsGet = c.queryLogsApi.apiV1ProfilesIdLogsBlocklistsGet as unknown as Mock;
const devicesGet = c.queryLogsApi.apiV1ProfilesIdLogsDevicesGet as unknown as Mock;
const profileGet = c.profilesApi.apiV1ProfilesIdGet as unknown as Mock;
const batchPost = c.profilesApi.apiV1ProfilesIdCustomRulesBatchPost as unknown as Mock;

function profile(subdomains: 'include' | 'exact' = 'exact'): ModelProfile {
    return {
        id: 'p1',
        profile_id: 'p1',
        account_id: 'a',
        name: 'Home',
        settings: {
            logs: { enabled: true, log_domains: true, log_clients_ips: true, retention: '1d' },
            statistics: { enabled: true, retention: '30d' },
            privacy: { custom_rules_subdomains_rule: subdomains },
        },
    } as unknown as ModelProfile;
}

function Probe() {
    const l = useLocation();
    return <div data-testid="loc">{l.pathname + l.search}</div>;
}

function mount(p = profile()) {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
    return render(
        <MemoryRouter initialEntries={['/statistics']}>
            <Probe />
            <Routes>
                <Route path="/statistics" element={<Statistics profiles={[p]} />} />
                <Route path="/custom-rules" element={<div>Custom rules page</div>} />
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
    resetQuickRuleSession();
    statsGet.mockResolvedValue({ data: createStatsResponse({ points: 48 }) });
    topGet.mockImplementation(async (_id: string, kind: string) => ({ data: kind === 'blocked' ? topBlocked : topResolved }));
    clientsGet.mockResolvedValue({ data: topClients });
    blocklistsGet.mockResolvedValue({ data: topBlocklists });
    devicesGet.mockResolvedValue({ data: devicesList });
    profileGet.mockResolvedValue({ data: profile() });
    batchPost.mockResolvedValue({ data: { created: [{}], skipped: [] } });
});

const card = async (name: string) => within(await screen.findByRole('region', { name }));
const ruleButton = (scope: ReturnType<typeof within>, domain: string) => scope.getByRole('button', { name: `Create a custom rule for ${domain}` });

describe('quick rules on the top domain lists', () => {
    it('puts a labelled button on every row in the table and the chart view', async () => {
        // tableRef: statistics-behaviour #P25, #X10
        const user = userEvent.setup();
        mount();
        const blocked = await card('Top blocked domains');
        expect(await blocked.findByRole('columnheader', { name: 'Actions' })).toBeInTheDocument();
        ruleButton(blocked, 'ads.example.com');
        ruleButton(blocked, 'tracker.example.net');
        await user.click(blocked.getByRole('button', { name: 'Chart' }));
        ruleButton(blocked, 'ads.example.com');
        ruleButton(blocked, 'tracker.example.net');
        const resolved = await card('Top resolved domains');
        ruleButton(resolved, 'example.org');
        ruleButton(resolved, 'cdn.example.com');
    });

    it('is quiet at rest and takes teal on the blocked card and red on the resolved card on hover or focus', async () => {
        // tableRef: statistics-behaviour #X10
        mount();
        const blocked = ruleButton(await card('Top blocked domains'), 'ads.example.com');
        const resolved = ruleButton(await card('Top resolved domains'), 'example.org');
        for (const b of [blocked, resolved]) {
            expect(b.className).toContain('text-[var(--stats-axis)]');
            expect(b.className).not.toMatch(/(^| )(bg|border)-/);
        }
        expect(blocked.className).toContain('group-hover:text-[var(--tailwind-colors-rdns-600)]');
        expect(blocked.className).toContain('group-focus-within:text-[var(--tailwind-colors-rdns-600)]');
        expect(resolved.className).toContain('group-hover:text-[var(--stats-blocked)]');
        expect(resolved.className).toContain('group-focus-within:text-[var(--stats-blocked)]');
    });

    it('strips the trailing dot from the domain in the label and the sheet', async () => {
        // tableRef: statistics-behaviour #P25
        const user = userEvent.setup();
        topGet.mockImplementation(async (_id: string, kind: string) => ({ data: { enabled: true, items: [{ domain: kind === 'blocked' ? 'dotted.example.' : 'x.example.', count: 3 }] } }));
        mount();
        await user.click(ruleButton(await card('Top blocked domains'), 'dotted.example'));
        expect(await screen.findByLabelText('Domain', { selector: 'input' })).toHaveValue('dotted.example');
    });

    it('opens the shared sheet with Allow on the blocked card and Block on the resolved card', async () => {
        // tableRef: statistics-behaviour #P25
        const user = userEvent.setup();
        mount();
        await user.click(ruleButton(await card('Top blocked domains'), 'ads.example.com'));
        expect(await screen.findByRole('radio', { name: 'Allow domain' })).toHaveAttribute('data-state', 'on');
        await user.click(screen.getByRole('button', { name: 'Cancel' }));
        await user.click(ruleButton(await card('Top resolved domains'), 'example.org'));
        await waitFor(() => expect(screen.getByRole('radio', { name: 'Block domain' })).toHaveAttribute('data-state', 'on'));
    });

    it('follows the profile subdomains setting for the value', async () => {
        // tableRef: statistics-behaviour #P25
        const user = userEvent.setup();
        mount(profile('include'));
        await user.click(ruleButton(await card('Top blocked domains'), 'ads.example.com'));
        expect(await screen.findByLabelText('Domain', { selector: 'input' })).toHaveValue('*.ads.example.com');
    });

    it('is disabled with the reason under limited access while rows stay readable', async () => {
        // tableRef: statistics-behaviour #P25, #X10
        const user = userEvent.setup();
        mount();
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        const blocked = await card('Top blocked domains');
        const button = ruleButton(blocked, 'ads.example.com');
        expect(button).toBeDisabled();
        expect(button).toHaveAccessibleDescription('Feature unavailable in limited access mode');
        expect(blocked.getByText('ads.example.com')).toBeInTheDocument();
        await user.click(button);
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('creates the rule, closes the sheet, toasts with a link, tags the row and returns focus', async () => {
        // tableRef: statistics-behaviour #P26, #X10
        const user = userEvent.setup();
        mount();
        const { toast } = await import('sonner');
        const blocked = await card('Top blocked domains');
        const button = ruleButton(blocked, 'ads.example.com');
        await user.click(button);
        await user.click(await screen.findByRole('button', { name: 'Add to Allowlist' }));
        await waitFor(() => expect(toast.success).toHaveBeenCalled());
        expect(batchPost).toHaveBeenCalledWith('p1', { action: 'allow', values: ['ads.example.com'] });
        const [message, options] = (toast.success as unknown as Mock).mock.calls[0];
        expect(message).toBe('ads.example.com added to the Allowlist.');
        expect(options.description).toBe('Past queries still count here - new queries follow the rule.');
        expect(options.action.label).toBe('View rules');
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(blocked.getByText('Rule added')).toBeInTheDocument();
        expect(blocked.getByText('ads.example.com')).toBeInTheDocument();
        await waitFor(() => expect(button).toHaveFocus());
        options.action.onClick();
        await waitFor(() => expect(screen.getByTestId('loc')).toHaveTextContent('/custom-rules?list=allowlist'));
    });

    it('opens the Denylist tab after a block rule from the resolved card, in the toast and the duplicate link', async () => {
        // tableRef: statistics-behaviour #P26
        const user = userEvent.setup();
        mount();
        const { toast } = await import('sonner');
        await user.click(ruleButton(await card('Top resolved domains'), 'example.org'));
        await user.click(await screen.findByRole('button', { name: 'Add to Denylist' }));
        await waitFor(() => expect(toast.success).toHaveBeenCalled());
        const [, options] = (toast.success as unknown as Mock).mock.calls[0];
        options.action.onClick();
        await waitFor(() => expect(screen.getByTestId('loc')).toHaveTextContent('/custom-rules?list=denylist'));
    });

    it('keeps the sheet open with an inline notice and a link when the rule already exists', async () => {
        // tableRef: statistics-behaviour #P26
        const user = userEvent.setup();
        batchPost.mockResolvedValue({ data: { created: [], skipped: [{ value: 'ads.example.com', reason: 'duplicate_existing', message: 'Rule already exists on this profile.' }] } });
        mount();
        await user.click(ruleButton(await card('Top blocked domains'), 'ads.example.com'));
        await user.click(await screen.findByRole('button', { name: 'Add to Allowlist' }));
        const alert = await screen.findByRole('alert');
        expect(alert).toHaveTextContent('A custom rule for ads.example.com already exists. Edit it in Custom rules.');
        expect(within(alert).getByRole('link', { name: 'Custom rules' })).toHaveAttribute('href', '/custom-rules?list=allowlist');
        expect(screen.getByRole('dialog')).toBeInTheDocument();
        const { toast } = await import('sonner');
        expect(toast.success).not.toHaveBeenCalled();
    });

    it('shows the server message and keeps the sheet open on an API error', async () => {
        // tableRef: statistics-behaviour #P26
        const user = userEvent.setup();
        batchPost.mockRejectedValue({ response: { data: { error: 'Adding these would exceed the limit of 10000 custom rules per profile. Remove some rules first.' } }, isAxiosError: true });
        mount();
        const blocked = await card('Top blocked domains');
        await user.click(ruleButton(blocked, 'ads.example.com'));
        await user.click(await screen.findByRole('button', { name: 'Add to Allowlist' }));
        const { toast } = await import('sonner');
        await waitFor(() => expect(toast.error).toHaveBeenCalled());
        expect(String((toast.error as unknown as Mock).mock.calls[0][0])).toContain('exceed the limit of 10000');
        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(screen.queryByText('Rule added')).not.toBeInTheDocument();
    });
});
