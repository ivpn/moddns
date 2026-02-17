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

    test('prefills domain with wildcard in default include mode', () => {
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="logs.example" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="logs.example" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('*.logs.example');
        expect(input).toHaveAttribute('readonly');
    });

    test('prefills domain verbatim in exact mode', () => {
        useAppStore.setState({
            activeProfile: {
                profile_id: 'profile-1',
                id: 'profile-1',
                settings: { privacy: { custom_rules_subdomains_rule: 'exact' } },
            } as any,
        });
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="ads.google.com" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="ads.google.com" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('ads.google.com');
    });

    test('strips www and adds wildcard in include mode', () => {
        useAppStore.setState({
            activeProfile: {
                profile_id: 'profile-1',
                id: 'profile-1',
                settings: { privacy: { custom_rules_subdomains_rule: 'include' } },
            } as any,
        });
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="www.google.com" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="www.google.com" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('*.google.com');
    });

    test('keeps www in exact mode', () => {
        useAppStore.setState({
            activeProfile: {
                profile_id: 'profile-1',
                id: 'profile-1',
                settings: { privacy: { custom_rules_subdomains_rule: 'exact' } },
            } as any,
        });
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="www.google.com" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="www.google.com" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('www.google.com');
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

    test('recalculates domain when activeProfile updates while sheet is open', () => {
        const noop = () => { };
        const { rerender } = render(<QuickRuleSheet open={false} domain="logs.example" onOpenChange={noop} />);
        act(() => {
            rerender(<QuickRuleSheet open domain="logs.example" onOpenChange={noop} />);
        });
        const input = screen.getByLabelText(/Domain/i, { selector: 'input' }) as HTMLInputElement;
        expect(input.value).toBe('*.logs.example');

        // Simulate activeProfile switching to exact mode while sheet is open
        act(() => {
            useAppStore.setState({
                activeProfile: {
                    profile_id: 'profile-1',
                    id: 'profile-1',
                    settings: { privacy: { custom_rules_subdomains_rule: 'exact' } },
                } as any,
            });
        });

        expect(input.value).toBe('logs.example');
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
