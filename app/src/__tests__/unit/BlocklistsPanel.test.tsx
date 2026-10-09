// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BlocklistsPanel } from '@/pages/statistics/panels/BlocklistsPanel';

const items = [
    { id: 'hagezi-pro', name: 'Hagezi Pro', count: 1200 },
    { id: 'unknown-list', count: 30 },
];

describe('BlocklistsPanel', () => {
    it('opens in table view with Chart one toggle away', async () => {
        // tableRef: statistics-behaviour #X7
        render(<BlocklistsPanel items={items} range="7d" windowWords="1 week" />);
        expect(screen.getByRole('table')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Table' })).toHaveAttribute('aria-pressed', 'true');
        await userEvent.setup().click(screen.getByRole('button', { name: 'Chart' }));
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
        expect(screen.getAllByTestId('bar-row')).toHaveLength(2);
    });

    it('ranks lists by their API name, falls back to the id when the name is missing, and states the multi-list counting', () => {
        // tableRef: statistics-behaviour #P22
        render(<BlocklistsPanel items={items} range="7d" windowWords="1 week" />);
        expect(screen.getByRole('heading', { name: 'Top blocklists' })).toBeInTheDocument();
        expect(screen.getByText('Hagezi Pro')).toBeInTheDocument();
        expect(screen.getByText('unknown-list')).toBeInTheDocument();
        expect(screen.getByText('A query blocked by several lists counts once for each.')).toBeInTheDocument();
    });

    it('the table view carries a count per list', () => {
        // tableRef: statistics-behaviour #P22
        render(<BlocklistsPanel items={items} range="7d" windowWords="1 week" />);
        const table = screen.getByRole('table');
        expect(within(table).getByRole('columnheader', { name: 'Blocklist' })).toBeInTheDocument();
        expect(within(table).getByRole('row', { name: /Hagezi Pro/ })).toHaveTextContent((1200).toLocaleString());
    });

    it('shows text and no toggle when nothing was blocked', () => {
        // tableRef: statistics-behaviour #P22, #P21
        render(<BlocklistsPanel items={[]} range="7d" windowWords="1 week" />);
        expect(screen.getByText('No blocked queries in this range.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Table' })).not.toBeInTheDocument();
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
    });
});
