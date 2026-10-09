// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DomainsPanel } from '@/pages/statistics/panels/DomainsPanel';

const items = [
    { domain: 'ads.example.com', count: 120 },
    { domain: 'tracker.example.net', count: 80 },
];

describe('DomainsPanel', () => {
    it.each(['blocked', 'resolved'] as const)('opens the %s list in table view with Chart one toggle away', async kind => {
        // tableRef: statistics-behaviour #X7
        render(<DomainsPanel kind={kind} items={items} range="7d" windowWords="1 week" />);
        expect(screen.getByRole('table')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Table' })).toHaveAttribute('aria-pressed', 'true');
        await userEvent.setup().click(screen.getByRole('button', { name: 'Chart' }));
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
        expect(screen.getAllByTestId('bar-row')).toHaveLength(2);
    });
});

describe('DomainsPanel trailing dot', () => {
    const dotted = [{ domain: 'example.com.', count: 5 }];

    it('shows names without the root dot in the table and the chart', async () => {
        // tableRef: statistics-behaviour #X7
        render(<DomainsPanel kind="blocked" items={dotted} range="7d" windowWords="1 week" />);
        expect(screen.getByRole('cell', { name: 'example.com' })).toBeInTheDocument();
        expect(screen.queryByText('example.com.')).not.toBeInTheDocument();
        await userEvent.setup().click(screen.getByRole('button', { name: 'Chart' }));
        expect(screen.getByText('example.com')).toHaveAttribute('title', 'example.com');
        expect(screen.queryByText('example.com.')).not.toBeInTheDocument();
    });
});
