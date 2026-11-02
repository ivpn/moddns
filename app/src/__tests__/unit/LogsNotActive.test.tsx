// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import LogsNotActive from '@/pages/logs/LogsNotActive';
import { useAppStore } from '@/store/general';
import type { ModelProfile } from '@/api/client';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/api/api', () => ({
    default: { Client: { profilesApi: { apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdGet: vi.fn() } } },
}));

const profile = (statsOn: boolean) =>
    ({
        id: 'p1',
        profile_id: 'p1',
        account_id: 'a',
        name: 'p1',
        settings: {
            logs: { enabled: false, log_domains: true, log_clients_ips: false, retention: '1h' },
            statistics: { enabled: statsOn },
        },
    }) as unknown as ModelProfile;

function renderIt(p: ModelProfile) {
    useAppStore.setState({ activeProfile: p, profiles: [p], subscriptionStatus: null });
    return render(
        <MemoryRouter initialEntries={['/query-logs']}>
            <Routes>
                <Route path="/query-logs" element={<LogsNotActive profile={p} />} />
                <Route path="/settings" element={<div>Settings page</div>} />
            </Routes>
        </MemoryRouter>,
    );
}

beforeEach(() => vi.clearAllMocks());

describe('LogsNotActive', () => {
    it('explains that nothing is stored when both are off and preselects Query logs', () => {
        // tableRef: statistics-behaviour #D3, #U1
        renderIt(profile(false));
        expect(screen.getByRole('heading', { name: 'Query logs are off' })).toBeInTheDocument();
        expect(screen.getByText('Nothing is stored for this profile. Turn on query logs to see each query here.')).toBeInTheDocument();
        expect(screen.getByRole('checkbox', { name: 'Query logs' })).toBeChecked();
        expect(screen.getByRole('checkbox', { name: 'Statistics' })).toBeChecked();
        expect(screen.getByText('Checked with query logs. Uncheck Statistics to keep query logs only.')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Turn on statistics and query logs' })).toBeEnabled();
    });

    it('says statistics are on when they are and stages only Query logs', () => {
        // tableRef: statistics-behaviour #D3, #U2
        renderIt(profile(true));
        expect(screen.getByText('Statistics are on for this profile. Turn on query logs to see each query.')).toBeInTheDocument();
        expect(screen.queryByText(/Checked with query logs/)).not.toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Turn on query logs' })).toBeEnabled();
    });

    it('offers all sub-options', () => {
        // tableRef: statistics-behaviour #D3
        renderIt(profile(false));
        expect(screen.getByRole('radiogroup', { name: 'Log domains' })).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Query logs Retention period' })).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'Statistics Retention period' })).toBeInTheDocument();
    });

    it('reaches Settings with the keyboard', async () => {
        const user = userEvent.setup();
        renderIt(profile(false));
        const link = screen.getByRole('button', { name: 'Go to settings' });
        link.focus();
        await user.keyboard('{Enter}');
        expect(screen.getByText('Settings page')).toBeInTheDocument();
    });
});
