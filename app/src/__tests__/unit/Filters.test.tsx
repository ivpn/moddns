import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, vi } from 'vitest';
import Filters from '@/pages/logs/Filters';

const baseProps = {
    searchInputValue: '',
    onSearchInputChange: vi.fn(),
    onSearchCommit: vi.fn(),
    onSearchClear: vi.fn(),
    committedSearchValue: '',
    onClearFilters: vi.fn(),
    filterValue: 'all',
    onFilterChange: vi.fn(),
    sortValue: 'created',
    onSortChange: vi.fn(),
    onRefresh: vi.fn(),
    timespanValue: undefined,
    onTimespanChange: vi.fn(),
    refreshIntervalKey: 'off' as const,
    onRefreshIntervalChange: vi.fn(),
    deviceIdValue: undefined,
    onDeviceIdChange: vi.fn(),
    availableDeviceIds: [],
};

const ACCENT = 'border-[var(--tailwind-colors-rdns-600)]';

describe('Filters', () => {
    it('gives every select trigger an accessible name', () => {
        render(<Filters {...baseProps} />);
        expect(screen.getByLabelText('Filter by status')).toBeInTheDocument();
        expect(screen.getByLabelText('Filter by device')).toBeInTheDocument();
        expect(screen.getByLabelText('Sort logs')).toBeInTheDocument();
        expect(screen.getByLabelText('Filter by timespan')).toBeInTheDocument();
    });

    it('shows no accents and no clear chip at defaults', () => {
        render(<Filters {...baseProps} />);
        for (const label of ['Filter by status', 'Filter by device', 'Sort logs', 'Filter by timespan']) {
            expect(screen.getByLabelText(label).className).not.toContain(ACCENT);
        }
        expect(screen.queryByTestId('logs-clear-filters')).not.toBeInTheDocument();
    });

    it('accents exactly the active triggers and shows the clear chip', () => {
        render(<Filters {...baseProps} filterValue="blocked" timespanValue="LAST_1_DAY" />);
        expect(screen.getByLabelText('Filter by status').className).toContain(ACCENT);
        expect(screen.getByLabelText('Filter by timespan').className).toContain(ACCENT);
        expect(screen.getByLabelText('Filter by device').className).not.toContain(ACCENT);
        expect(screen.getByLabelText('Sort logs').className).not.toContain(ACCENT);

        const chip = screen.getByTestId('logs-clear-filters');
        fireEvent.click(chip);
        expect(baseProps.onClearFilters).toHaveBeenCalled();
    });

    it('a committed search shows the clear chip; uncommitted typing does not', () => {
        const { rerender } = render(<Filters {...baseProps} searchInputValue="typing" />);
        expect(screen.queryByTestId('logs-clear-filters')).not.toBeInTheDocument();

        rerender(<Filters {...baseProps} searchInputValue="typing" committedSearchValue="typing" />);
        expect(screen.getByTestId('logs-clear-filters')).toBeInTheDocument();
    });

    it('search inputs expose a clear button only when text is present', () => {
        const { rerender } = render(<Filters {...baseProps} />);
        expect(screen.queryAllByTestId('logs-search-clear')).toHaveLength(0);

        rerender(<Filters {...baseProps} searchInputValue="abc" />);
        // One per breakpoint instance (mobile row + desktop row).
        const clears = screen.getAllByTestId('logs-search-clear');
        expect(clears.length).toBeGreaterThan(0);
        fireEvent.click(clears[0]);
        expect(baseProps.onSearchClear).toHaveBeenCalled();
    });

    it('search commits on Enter, and blur alone does not commit', () => {
        render(<Filters {...baseProps} searchInputValue="abc" />);
        const inputs = screen.getAllByLabelText('Search domain or its part');
        fireEvent.blur(inputs[0]);
        expect(baseProps.onSearchCommit).not.toHaveBeenCalled();
        fireEvent.keyDown(inputs[0], { key: 'Enter' });
        expect(baseProps.onSearchCommit).toHaveBeenCalledTimes(1);
    });
});
