import { describe, it, expect, vi, beforeEach } from 'vitest';
// Ensure jsdom environment
// @vitest-environment jsdom
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ChangeEmailDialog from '@/pages/account_preferences/ChangeEmailDialog';
import type { ModelAccount, ResponsesPasskey } from '@/api/client/api';
import { useAppStore } from '@/store/general';
import api from '@/api/api';

vi.mock('@/api/api', () => {
    return {
        default: {
            Client: {
                accountsApi: {
                    apiV1AccountsPatch: vi.fn().mockResolvedValue({}),
                    apiV1AccountsCurrentGet: vi.fn().mockResolvedValue({ data: { email: 'new@example.com', email_verified: false } })
                }
            }
        }
    };
});

// simple helper to access store
const getAccount = () => useAppStore.getState().account;

// Fill step 1 (password method) and advance to the confirm step.
const fillDetailsAndContinue = () => {
    fireEvent.click(screen.getByText('Password'));
    fireEvent.change(screen.getByPlaceholderText('new@example.com'), { target: { value: 'new@example.com' } });
    fireEvent.change(screen.getByLabelText('Current password'), { target: { value: 'password123' } });
    fireEvent.click(screen.getByText('Continue'));
};

describe('ChangeEmailDialog', () => {
    beforeEach(() => {
        vi.mocked(api.Client.accountsApi.apiV1AccountsPatch).mockClear();
        useAppStore.getState().setAccount({ email: 'old@example.com', email_verified: true } as ModelAccount);
        useAppStore.getState().setPasskeys([]);
    });

    it('prefers passkey verification when available', () => {
        useAppStore.getState().setPasskeys([
            { id: 'cred-1' } as unknown as ResponsesPasskey,
        ]);
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={() => { }} />);

        expect(screen.getByText('Verify with passkey')).toBeInTheDocument();
        expect(screen.queryByLabelText('Current password')).not.toBeInTheDocument();
    });

    it('advances to a confirm step that shows the lockout warning', () => {
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={() => { }} />);
        fillDetailsAndContinue();

        expect(screen.getByTestId('input-confirm-email')).toBeInTheDocument();
        const warning = screen.getByTestId('confirm-email-warning');
        expect(warning.textContent).toMatch(/password reset/i);
        expect(warning.textContent).toMatch(/lose access|locked out/i);
        // Submit must not have fired yet.
        expect(api.Client.accountsApi.apiV1AccountsPatch).not.toHaveBeenCalled();
    });

    it('submits only after the confirm entry matches (case/whitespace-insensitive)', async () => {
        const onChanged = vi.fn();
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={onChanged} />);
        fillDetailsAndContinue();

        // A casing/padding variant of the same address counts as a match.
        fireEvent.change(screen.getByTestId('input-confirm-email'), { target: { value: ' NEW@Example.com ' } });
        fireEvent.click(screen.getByText('Update email'));

        await waitFor(() => expect(onChanged).toHaveBeenCalled());
        expect(api.Client.accountsApi.apiV1AccountsPatch).toHaveBeenCalledTimes(1);
        expect(getAccount()?.email).toBe('new@example.com');
        expect(getAccount()?.email_verified).toBe(false);
    });

    // specRef: api-endpoint-behaviour B3, E6
    it('with 2FA on, sends the code in the body next to the password', async () => {
        useAppStore.getState().setAccount({ email: 'old@example.com', email_verified: true, mfa: { totp: { enabled: true } } } as ModelAccount);
        const onChanged = vi.fn();
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={onChanged} />);
        fireEvent.change(screen.getByPlaceholderText('new@example.com'), { target: { value: 'new@example.com' } });
        fireEvent.change(screen.getByLabelText('Current password'), { target: { value: 'password123' } });
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '123456' } });
        await waitFor(() => expect(screen.getByText('Continue')).toBeEnabled());
        fireEvent.click(screen.getByText('Continue'));
        fireEvent.change(screen.getByTestId('input-confirm-email'), { target: { value: 'new@example.com' } });
        fireEvent.click(screen.getByText('Update email'));

        await waitFor(() => expect(onChanged).toHaveBeenCalled());
        const call = vi.mocked(api.Client.accountsApi.apiV1AccountsPatch).mock.calls[0];
        expect(call).toHaveLength(1);
        expect(call[0]).toMatchObject({
            otp: '123456',
            updates: [{ value: { current_password: 'password123', new_email: 'new@example.com' } }],
        });
    });

    it('blocks submit and shows an error while the confirm entry mismatches', () => {
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={() => { }} />);
        fillDetailsAndContinue();

        fireEvent.change(screen.getByTestId('input-confirm-email'), { target: { value: 'wrong@example.com' } });

        expect(screen.getByTestId('confirm-email-mismatch')).toBeInTheDocument();
        const submit = screen.getByText('Update email').closest('button');
        expect(submit).toBeDisabled();
        fireEvent.click(screen.getByText('Update email'));
        expect(api.Client.accountsApi.apiV1AccountsPatch).not.toHaveBeenCalled();
    });

    it('back returns to the first step with the entered address preserved', () => {
        render(<ChangeEmailDialog open account={getAccount()} onOpenChange={() => { }} onChanged={() => { }} />);
        fillDetailsAndContinue();

        fireEvent.click(screen.getByText('Back'));

        expect(screen.getByPlaceholderText('new@example.com')).toHaveValue('new@example.com');
        expect(screen.queryByTestId('input-confirm-email')).not.toBeInTheDocument();
    });
});
