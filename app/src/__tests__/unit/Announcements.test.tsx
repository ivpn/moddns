import { act, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import Announcements from '@/pages/announcements/Announcements';
import api from '@/api/api';

// Minimal IntersectionObserver mock so the infinite-scroll sentinel can be
// fired manually (jsdom has no IntersectionObserver).
class MockIntersectionObserver {
    callback: IntersectionObserverCallback;
    static lastInstance: MockIntersectionObserver | null = null;
    constructor(callback: IntersectionObserverCallback) {
        this.callback = callback;
        MockIntersectionObserver.lastInstance = this;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
    trigger(entries: IntersectionObserverEntry[]) {
        this.callback(entries, this as unknown as IntersectionObserver);
    }
}

declare global {
    // eslint-disable-next-line no-var
    var IntersectionObserver: typeof MockIntersectionObserver;
}

global.IntersectionObserver = MockIntersectionObserver as unknown as typeof globalThis.IntersectionObserver;

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
        MockIntersectionObserver.lastInstance = null;
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

    it('renders only the first chunk and reveals more on scroll', async () => {
        // PAGE_SIZE is 15; provide 20 so a second chunk exists.
        const TOTAL = 20;
        mockGet.mockResolvedValueOnce({
            data: Array.from({ length: TOTAL }, (_, i) => ({
                id: `a${i}`,
                category: 'news',
                severity: 'info',
                title: `Announcement ${i}`,
                published_at: '2026-05-28T00:00:00Z',
            })),
        });

        renderPage();

        // First chunk (0..14) is rendered; the 16th item (index 15) is not yet.
        expect(await screen.findByText('Announcement 0')).toBeInTheDocument();
        expect(screen.getByText('Announcement 14')).toBeInTheDocument();
        expect(screen.queryByText('Announcement 15')).not.toBeInTheDocument();

        // Scrolling the sentinel into view reveals the next chunk (all remaining).
        act(() => {
            MockIntersectionObserver.lastInstance?.trigger([
                { isIntersecting: true } as IntersectionObserverEntry,
            ]);
        });

        expect(await screen.findByText('Announcement 15')).toBeInTheDocument();
        expect(screen.getByText(`Announcement ${TOTAL - 1}`)).toBeInTheDocument();
    });
});
