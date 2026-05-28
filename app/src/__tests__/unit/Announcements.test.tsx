import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Announcements from '@/pages/announcements/Announcements';
import api from '@/api/api';

vi.mock('@/components/theme-provider', () => ({
    useTheme: () => ({ theme: 'dark', setTheme: vi.fn() }),
}));

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            announcementsApi: {
                apiV1AnnouncementsGet: vi.fn(),
            },
        },
    },
}));

const mockGet = api.Client.announcementsApi.apiV1AnnouncementsGet as unknown as ReturnType<typeof vi.fn>;

const renderPage = () =>
    render(
        <MemoryRouter>
            <Announcements />
        </MemoryRouter>
    );

describe('Announcements page', () => {
    beforeEach(() => {
        mockGet.mockReset();
    });

    it('renders announcements with category, date, markdown body and link', async () => {
        mockGet.mockResolvedValueOnce({
            data: [
                {
                    id: 'a1',
                    category: 'maintenance',
                    severity: 'warning',
                    title: 'EU resolver maintenance',
                    body: 'First announcement body.',
                    published_at: '2026-05-28T00:00:00Z',
                    link: 'https://example.com/status',
                },
            ],
        });

        renderPage();

        expect(await screen.findByText('EU resolver maintenance')).toBeInTheDocument();
        expect(screen.getByText('Maintenance')).toBeInTheDocument();
        expect(screen.getByText('May 28, 2026')).toBeInTheDocument();
        expect(screen.getByText('First announcement body.')).toBeInTheDocument();

        const link = screen.getByText('Learn more').closest('a');
        expect(link).toHaveAttribute('href', 'https://example.com/status');
    });

    it('shows the empty state when there are no announcements', async () => {
        mockGet.mockResolvedValueOnce({ data: [] });
        renderPage();
        expect(await screen.findByText('No announcements at the moment.')).toBeInTheDocument();
    });

    it('shows an error message when the request fails', async () => {
        mockGet.mockRejectedValueOnce(new Error('network'));
        renderPage();
        await waitFor(() =>
            expect(
                screen.getByText('Unable to load announcements right now. Please try again later.')
            ).toBeInTheDocument()
        );
    });
});
