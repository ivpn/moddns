// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { routePreload } from '@/App';
import BottomNav from '@/components/navigation/BottomNav';
import NavigationMenu from '@/pages/navigation_menu/NavigationMenu';
import { NavigationCollapseProvider } from '@/context/NavigationCollapseContext';

vi.mock('@/api/api', () => ({
    default: { Client: { announcementsApi: { apiV1AnnouncementsGet: vi.fn().mockResolvedValue({ data: [] }) } } },
}));

function names(el: HTMLElement) {
    return within(el).getAllByRole('button').map(b => b.textContent?.trim());
}

describe('navigation', () => {
    it('bottom nav lists Blocklists, Rules, Statistics, Logs, More', () => {
        // tableRef: statistics-behaviour #N3
        render(
            <MemoryRouter initialEntries={['/statistics']}>
                <BottomNav onMoreClick={() => {}} />
            </MemoryRouter>,
        );
        const nav = screen.getByTestId('bottom-nav');
        expect(names(nav)).toEqual(['Blocklists', 'Rules', 'Statistics', 'Logs', 'More']);
        expect(screen.queryByRole('button', { name: 'Setup' })).not.toBeInTheDocument();
    });

    it('bottom nav marks Statistics as the active route on /statistics', () => {
        // tableRef: statistics-behaviour #N3
        render(
            <MemoryRouter initialEntries={['/statistics']}>
                <BottomNav onMoreClick={() => {}} />
            </MemoryRouter>,
        );
        expect(screen.getByRole('button', { name: 'Statistics' }).className).toContain('text-[var(--tailwind-colors-rdns-600)]');
        expect(screen.getByRole('button', { name: 'Logs' }).className).toContain('text-muted-foreground');
    });

    it('the menu orders DNS Setup, Blocklists, Custom rules, Statistics, Logs, Settings, Account', () => {
        // tableRef: statistics-behaviour #N2
        render(
            <MemoryRouter initialEntries={['/statistics']}>
                <NavigationCollapseProvider>
                    <NavigationMenu isMobile={true} onClose={() => {}} />
                </NavigationCollapseProvider>
            </MemoryRouter>,
        );
        const labels = within(screen.getByRole('navigation', { name: 'Primary' }))
            .getAllByRole('button')
            .map(b => b.textContent?.trim().replace(/New$/, '').trim())
            .filter(t => ['DNS Setup', 'Blocklists', 'Custom rules', 'Statistics', 'Logs', 'Settings', 'Account'].includes(t ?? ''));
        expect(labels).toEqual(['DNS Setup', 'Blocklists', 'Custom rules', 'Statistics', 'Logs', 'Settings', 'Account']);
    });

    it('marks only Statistics as new, announced as "Statistics, new"', () => {
        // tableRef: statistics-behaviour #N2
        render(
            <MemoryRouter initialEntries={['/setup']}>
                <NavigationCollapseProvider>
                    <NavigationMenu isMobile={true} onClose={() => {}} />
                </NavigationCollapseProvider>
            </MemoryRouter>,
        );
        const pills = screen.getAllByTestId('nav-new-pill');
        expect(pills).toHaveLength(1);
        expect(pills[0]).toHaveAttribute('aria-hidden', 'true');
        expect(screen.getByRole('button', { name: 'Statistics, new' })).toContainElement(pills[0]);
        expect(screen.getByRole('button', { name: 'Logs' })).toBeInTheDocument();
    });

    it('bottom nav carries no new pill', () => {
        render(
            <MemoryRouter initialEntries={['/statistics']}>
                <BottomNav onMoreClick={() => {}} />
            </MemoryRouter>,
        );
        expect(screen.queryByTestId('nav-new-pill')).not.toBeInTheDocument();
    });

    it('every navigation route has a preloader', () => {
        // tableRef: statistics-behaviour #N1
        for (const route of ['/setup', '/blocklists', '/custom-rules', '/statistics', '/query-logs', '/settings']) {
            expect(routePreload[route], route).toBeTypeOf('function');
        }
    });
});
