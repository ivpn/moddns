import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import CustomRulesCard, { type CustomRulesCardProps } from '@/pages/custom_rules/CustomRulesCard';
import { useAppStore } from '@/store/general';
import type { ModelCustomRule, ModelProfile } from '@/api/client/api';

const PROFILE_ID = 'p1';

function makeRules(count: number, group = '', prefix = 'r'): ModelCustomRule[] {
    return Array.from({ length: count }, (_, i) => ({
        id: `${prefix}${i}`,
        action: 'block',
        value: `${prefix}${i}.example.com`,
        group,
        order: i,
    }) as ModelCustomRule);
}

function renderCard(rules: ModelCustomRule[], groupOrder: string[] = []) {
    const noop = vi.fn();
    const props: CustomRulesCardProps = {
        rules,
        groupNotes: {},
        groupOrder,
        selectedIds: [],
        onCheck: noop,
        onDelete: noop,
        onEdit: noop,
        onReorder: noop,
        onMoveRule: noop,
        onSaveGroupNote: noop,
        onCreateGroup: noop,
        onRenameGroup: noop,
        onDeleteGroup: noop,
        onReorderGroups: noop,
        allSelected: false,
        selectedCount: 0,
        handleBulkDelete: noop,
        loading: false,
        type: 'denied',
        searchQuery: '',
    };
    return render(<CustomRulesCard {...props} />);
}

// Selector queries: role queries walk the accessibility tree of every row and are slow at this size.
const rowCount = () => document.querySelectorAll('[aria-label="Edit rule"]').length;
const showMore = (n: number) => screen.queryByText(`Show ${n} more`);

// These render a few hundred rows in jsdom; the default 5s is tight under full-suite load.
describe('CustomRulesCard rendering budget', { timeout: 20_000 }, () => {
    beforeEach(() => {
        useAppStore.setState({
            activeProfile: { id: PROFILE_ID } as ModelProfile,
            customRulesCollapsed: {},
        });
    });

    it('renders at most 200 rules per section and reveals the rest on request', () => {
        renderCard(makeRules(250));
        expect(rowCount()).toBe(200);

        fireEvent.click(showMore(50)!);
        expect(rowCount()).toBe(250);
        expect(screen.queryByText(/^Show \d+ more$/)).not.toBeInTheDocument();
    });

    it('states how many rules are not shown and offers the next 200', () => {
        renderCard(makeRules(450));
        expect(screen.getByText(/250 rules not shown/)).toBeInTheDocument();
        expect(showMore(200)).toBeInTheDocument();
    });

    it('suggests groups only when the Ungrouped section is cut short', () => {
        const { unmount } = renderCard(makeRules(201));
        expect(screen.getByText(/put rules into groups/)).toBeInTheDocument();
        unmount();

        renderCard([...makeRules(10), ...makeRules(300, 'work', 'w')], ['work']);
        expect(screen.queryByText(/put rules into groups/)).not.toBeInTheDocument();
        expect(showMore(100)).toBeInTheDocument();
    });

    it('does not render the rows of a collapsed group', () => {
        useAppStore.setState({ customRulesCollapsed: { [`${PROFILE_ID}:denied`]: ['work'] } });
        renderCard([...makeRules(3), ...makeRules(5, 'work', 'w')], ['work']);
        expect(rowCount()).toBe(3);
        expect(screen.queryByText('w0.example.com')).not.toBeInTheDocument();
    });

    it('mounts a group\'s rows when it opens and removes them after it closes', async () => {
        useAppStore.setState({ customRulesCollapsed: { [`${PROFILE_ID}:denied`]: ['work'] } });
        renderCard(makeRules(5, 'work', 'w'), ['work']);
        expect(rowCount()).toBe(0);

        fireEvent.click(screen.getByRole('button', { name: /^work/ }));
        expect(rowCount()).toBe(5);

        fireEvent.click(screen.getByRole('button', { name: /^work/ }));
        // Rows stay for the close animation, then leave the page.
        expect(rowCount()).toBe(5);
        await waitFor(() => expect(rowCount()).toBe(0));
    });
});
