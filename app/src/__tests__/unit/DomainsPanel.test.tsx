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
