import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, test, expect, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import QueryLogsSection from '@/pages/settings/QueryLogsSection';
import type { ModelProfile } from '@/api/client';
import { useAppStore } from '@/store/general';

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: { apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdGet: vi.fn().mockRejectedValue(new Error('offline')) },
            queryLogsApi: {},
            statisticsApi: { apiV1ProfilesIdStatisticsDelete: vi.fn() },
        },
    },
}));

const profile = {
    id: 'p1',
    profile_id: 'p1',
    account_id: 'a',
    name: 'p1',
    settings: {
        logs: { enabled: true, log_domains: true, log_clients_ips: false, retention: '1d' },
        statistics: { enabled: true },
    },
} as unknown as ModelProfile;

describe('QueryLogsSection', () => {
    test('keeps Clear query logs and Delete statistics history available under limited access', () => {
        // tableRef: statistics-behaviour #S5
        useAppStore.setState({ subscriptionStatus: 'limited_access' });
        try {
            render(<MemoryRouter><QueryLogsSection activeProfile={profile} /></MemoryRouter>);
            expect(screen.getByRole('button', { name: 'Clear query logs' })).toBeEnabled();
            expect(screen.getByRole('button', { name: 'Delete statistics history' })).toBeEnabled();
        } finally {
            useAppStore.setState({ subscriptionStatus: null });
        }
    });

    test('renders the Data collection heading, intro and Download / Clear actions', () => {
        // tableRef: statistics-behaviour #D1, #C1
        render(<MemoryRouter><QueryLogsSection activeProfile={profile} /></MemoryRouter>);
        expect(screen.getByRole('heading', { name: 'DATA COLLECTION' })).toBeInTheDocument();
        expect(screen.getByText("Choose what modDNS keeps about this profile's DNS queries. Off by default.")).toBeInTheDocument();
        expect(screen.getByRole('group', { name: 'DATA COLLECTION' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Download query logs' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Clear query logs' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Delete statistics history' })).toBeInTheDocument();
    });

    test('shows Delete statistics history only while statistics are saved on, behind a confirmation', async () => {
        // tableRef: statistics-behaviour #S3, #D1
        const off = { ...profile, settings: { ...profile.settings, statistics: { enabled: false } } } as unknown as ModelProfile;
        const { unmount } = render(<MemoryRouter><QueryLogsSection activeProfile={off} /></MemoryRouter>);
        expect(screen.queryByRole('button', { name: 'Delete statistics history' })).not.toBeInTheDocument();
        unmount();
        render(<MemoryRouter><QueryLogsSection activeProfile={profile} /></MemoryRouter>);
        fireEvent.click(screen.getByRole('button', { name: 'Delete statistics history' }));
        const dialog = await screen.findByRole('dialog', { name: 'Delete statistics history?' });
        expect(within(dialog).getByRole('button', { name: 'Delete history' })).toBeInTheDocument();
    });

    test('the #data-collection deep link lands on the statistics retention', async () => {
        // tableRef: statistics-behaviour #S4
        const { container } = render(<MemoryRouter initialEntries={['/settings#data-collection']}><QueryLogsSection activeProfile={profile} /></MemoryRouter>);
        expect(container.querySelector('#data-collection')).not.toBeNull();
        const pills = screen.getByRole('radiogroup', { name: 'Statistics Retention period' });
        await waitFor(() => expect(within(pills).getByLabelText('30 days')).toHaveFocus());
    });
});
