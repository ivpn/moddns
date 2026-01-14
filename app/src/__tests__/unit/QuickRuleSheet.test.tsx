import { describe, beforeEach, afterEach, test, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { act } from 'react-dom/test-utils';
import '@testing-library/jest-dom';
import QuickRuleSheet from '@/pages/logs/QuickRuleSheet';
import { useAppStore } from '@/store/general';

describe('QuickRuleSheet', () => {
    beforeEach(() => {
        useAppStore.setState({
            activeProfile: {
                profile_id: 'profile-1',
                id: 'profile-1',
                settings: {},
            } as any,
        });
    });

    afterEach(() => {
        useAppStore.setState({ activeProfile: null });
    });

    test('prefills domain field when opened', () => {
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="logs.example" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="logs.example" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('logs.example');
    });

    test('updates action label when toggled and closes on cancel', () => {
        const onOpenChange = vi.fn();
        const { rerender } = render(<QuickRuleSheet open={false} domain="logs.example" onOpenChange={onOpenChange} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="logs.example" onOpenChange={onOpenChange} />);
        });
        const allowToggle = screen.getByRole('radio', { name: /Allow domain/i });
        fireEvent.click(allowToggle);
        expect(screen.getByText('Add to Allowlist')).toBeInTheDocument();
        const cancelButton = screen.getByRole('button', { name: 'Cancel' });
        fireEvent.click(cancelButton);
        expect(onOpenChange).toHaveBeenCalledWith(false);
    });

    test('uses provided defaultAction when opening', () => {
        const noop = () => { };
        const { rerender } = render(
            <QuickRuleSheet open={false} domain="logs.example" onOpenChange={noop} defaultAction="allowlist" />
        );
        act(() => {
            rerender(
                <QuickRuleSheet open domain="logs.example" onOpenChange={noop} defaultAction="allowlist" />
            );
        });
        expect(screen.getByText('Add to Allowlist')).toBeInTheDocument();
        const allowToggle = screen.getByRole('radio', { name: /Allow domain/i });
        expect(allowToggle).toHaveAttribute('data-state', 'on');
    });
});
