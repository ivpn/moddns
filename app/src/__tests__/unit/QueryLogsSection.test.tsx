import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, test, expect, vi } from 'vitest';
import QueryLogsSection from '@/pages/settings/QueryLogsSection';
import type { ModelProfile } from '@/api/client';

vi.mock('@/api/api', () => ({
    default: { Client: { profilesApi: { apiV1ProfilesIdPatch: vi.fn(), apiV1ProfilesIdGet: vi.fn() }, queryLogsApi: {} } },
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
    test('renders the Data collection heading, intro and Download / Clear actions', () => {
        // tableRef: statistics-behaviour #D1, #C1
        render(<QueryLogsSection activeProfile={profile} />);
        expect(screen.getByRole('heading', { name: 'DATA COLLECTION' })).toBeInTheDocument();
        expect(screen.getByText("Choose what modDNS keeps about this profile's DNS queries. Off by default.")).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', { name: 'DATA COLLECTION' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Download query logs' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Clear query logs' })).toBeInTheDocument();
    });
});

describe('QueryLogsSection retention info tooltip', () => {
    test('shows informational tooltip content when hovering info icon', async () => {
        render(<QueryLogsSection activeProfile={profile} />);
        const trigger = screen.getByTestId('retention-info-trigger');
        expect(trigger).toBeInTheDocument();
        fireEvent.mouseEnter(trigger);
        // Tooltip uses setTimeout even with delay=0; wait for appearance
        const tooltipText = await screen.findByText(/Changing the retention period switches to a new set of query logs/i, {}, { timeout: 500 });
        expect(tooltipText).toBeVisible();
    });

    test('accessible name on trigger button', () => {
        render(<QueryLogsSection activeProfile={profile} />);
        expect(screen.getByRole('button', { name: /Retention period information/i })).toBeInTheDocument();
    });
});
