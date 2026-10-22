// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BlocklistsPanel } from '@/pages/statistics/panels/BlocklistsPanel';
import { withEnabledLists } from '@/pages/statistics/derive';

const items = [
    { id: 'hagezi-pro', name: 'Hagezi Pro', count: 1200 },
    { id: 'unknown-list', count: 30 },
];

describe('BlocklistsPanel', () => {
    it('opens in table view with Chart one toggle away', async () => {
        // tableRef: statistics-behaviour #X7
        render(<BlocklistsPanel items={items} scopeIn="in the last 7 days" />);
        expect(screen.getByRole('table')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Table' })).toHaveAttribute('aria-pressed', 'true');
        await userEvent.setup().click(screen.getByRole('button', { name: 'Chart' }));
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
        expect(screen.getAllByTestId('bar-row')).toHaveLength(2);
    });

    it('shows every loaded blocklist in Chart view, without a show more/less toggle', async () => {
        // tableRef: statistics-behaviour #X7
        const ten = Array.from({ length: 10 }, (_, i) => ({ id: `list-${i}`, name: `List ${i}`, count: 100 - i }));
        render(<BlocklistsPanel items={ten} scopeIn="in the last 7 days" />);
        await userEvent.setup().click(screen.getByRole('button', { name: 'Chart' }));
        expect(screen.getAllByTestId('bar-row')).toHaveLength(10);
        expect(screen.queryByRole('button', { name: /^Show / })).not.toBeInTheDocument();
    });

    it('ranks lists by their API name, falls back to the id when the name is missing, and states how a query is credited', () => {
        // tableRef: statistics-behaviour #P22
        render(<BlocklistsPanel items={items} scopeIn="in the last 7 days" />);
        expect(screen.getByRole('heading', { name: 'Top blocklists' })).toBeInTheDocument();
        expect(screen.getByText('Hagezi Pro')).toBeInTheDocument();
        expect(screen.getByText('unknown-list')).toBeInTheDocument();
        expect(
            screen.getByText('Each blocked query counts for one list only: the earliest-enabled list that blocks it. Lists that overlap with earlier ones can show few or no hits.'),
        ).toBeInTheDocument();
        expect(screen.queryByText(/counts once for each/)).not.toBeInTheDocument();
    });

    it('lists enabled lists without hits as 0 after the ranked ones', () => {
        // tableRef: statistics-behaviour #P22
        render(<BlocklistsPanel items={[...items, { id: 'steven', name: 'Steven Black', count: 0 }]} scopeIn="in the last 7 days" />);
        const rows = within(screen.getByRole('table')).getAllByRole('row').slice(1);
        expect(rows.map(r => within(r).getAllByRole('cell').map(c => c.textContent))).toEqual([
            ['1', 'Hagezi Pro', '1,200'],
            ['2', 'unknown-list', '30'],
            ['3', 'Steven Black', '0'],
        ]);
    });

    it('keeps the no-hits text when no enabled list blocked anything', () => {
        // tableRef: statistics-behaviour #P22, #P27
        render(<BlocklistsPanel items={[{ id: 'steven', name: 'Steven Black', count: 0 }]} scopeIn="in the last 7 days" />);
        expect(screen.getByText('No queries blocked by a blocklist in the last 7 days.')).toBeInTheDocument();
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
    });
});

describe('withEnabledLists', () => {
    const hits = [{ id: 'a', name: 'A list', count: 5 }];

    it('adds enabled lists missing from the hits as 0, named from the catalog and sorted by name', () => {
        // tableRef: statistics-behaviour #P22
        const names = new Map([['z', 'Zeta'], ['m', 'Mu']]);
        expect(withEnabledLists(hits, ['a', 'z', 'm', 'x'], names)).toEqual([
            { id: 'a', name: 'A list', count: 5 },
            { id: 'm', name: 'Mu', count: 0 },
            { id: 'x', name: undefined, count: 0 },
            { id: 'z', name: 'Zeta', count: 0 },
        ]);
    });

    it('adds nothing when the hits were cut off at the top-list limit', () => {
        // tableRef: statistics-behaviour #P22, #X7
        const full = Array.from({ length: 50 }, (_, i) => ({ id: `l${i}`, count: 100 - i }));
        expect(withEnabledLists(full, ['l0', 'other'], new Map())).toBe(full);
    });

    it('captions the table with the logs scope', () => {
        // tableRef: statistics-behaviour #P27
        render(<BlocklistsPanel items={items} scopeIn="in the last 7 days" />);
        expect(screen.getByText('Top blocklists, from query logs in the last 7 days')).toBeInTheDocument();
    });

    it('the table view carries a count per list', () => {
        // tableRef: statistics-behaviour #P22
        render(<BlocklistsPanel items={items} scopeIn="in the last 7 days" />);
        const table = screen.getByRole('table');
        expect(within(table).getByRole('columnheader', { name: 'Blocklist' })).toBeInTheDocument();
        expect(within(table).getByRole('row', { name: /Hagezi Pro/ })).toHaveTextContent((1200).toLocaleString());
    });

    it('shows text and no toggle when nothing was blocked', () => {
        // tableRef: statistics-behaviour #P22, #P21, #P27
        render(<BlocklistsPanel items={[]} scopeIn="in the last 7 days" />);
        expect(screen.getByText('No queries blocked by a blocklist in the last 7 days.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Table' })).not.toBeInTheDocument();
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
    });
});
