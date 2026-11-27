import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import UpdatePasswordDialog from '@/pages/account_preferences/UpdatePasswordDialog';
import { useAppStore } from '@/store/general';

// Mock api client and toast
vi.mock('sonner', () => ({
    toast: {
        error: vi.fn(),
        success: vi.fn()
    }
}));

const patchMock = vi.fn().mockResolvedValue({});
const reauthMock = vi.fn();
const currentGetMock = vi.fn().mockResolvedValue({ data: { account_id: 'abc', mfa: { totp: { enabled: false } } } });
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            accountsApi: {
                apiV1AccountsPatch: (...args: unknown[]) => patchMock(...args),
                apiV1AccountsReauthPost: (...args: unknown[]) => reauthMock(...args),
                apiV1AccountsCurrentGet: () => currentGetMock()
            }
        }
    }
}));

const beginPasswordSetReauthMock = vi.fn().mockResolvedValue('reauth-token');
vi.mock('@/lib/webauthn', () => ({
    beginPasswordSetReauth: () => beginPasswordSetReauthMock(),
}));

// Helper to set account state in zustand store
const setAccountState = (account: Record<string, unknown>) => {
    const setAccount = useAppStore.getState().setAccount;
    setAccount(account);
};

describe('UpdatePasswordDialog', () => {
    beforeEach(async () => {
        patchMock.mockReset();
        currentGetMock.mockReset();
        reauthMock.mockReset().mockResolvedValue({ data: { reauth_token: 'password-token', expires_at: new Date(Date.now() + 300_000).toISOString() } });
        const { toast } = await import('sonner');
        (toast.error as unknown as { mockReset?: () => void }).mockReset?.();
        (toast.success as unknown as { mockReset?: () => void }).mockReset?.();
        // default account with no 2FA
        setAccountState({ account_id: 'abc', mfa: { totp: { enabled: false } } });
    });

    it('blocks submission when old password missing', async () => {
        render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

        // Fill only new + confirm passwords
        fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
        fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
        fireEvent.click(screen.getByText('Save change'));

        await waitFor(() => {
            expect(patchMock).not.toHaveBeenCalled();
        });
    });

    // specRef: api-endpoint-behaviour B3a, B15
    it('exchanges the old password for a token and submits test + replace', async () => {
        render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

        fireEvent.change(screen.getByLabelText('Old password'), { target: { value: 'OldPassword123!' } });
        fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
        fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
        fireEvent.click(screen.getByText('Save change'));

        await waitFor(() => {
            expect(patchMock).toHaveBeenCalledTimes(1);
            expect(reauthMock.mock.calls).toEqual([[{ purpose: 'password_set', current_password: 'OldPassword123!' }]]);
            expect(patchMock.mock.calls[0]).toEqual([{
                updates: [
                    { operation: 'test', path: '/password', value: { reauth_token: 'password-token' } },
                    { operation: 'replace', path: '/password', value: 'NewPassword123!' },
                ],
            }]);
        });
    });

    // specRef: api-endpoint-behaviour B3a, B15
    it('requires OTP when 2FA enabled and sends it with the old password for the token', async () => {
        // Enable 2FA in store
        setAccountState({ account_id: 'abc', mfa: { totp: { enabled: true } } });
        render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

        fireEvent.change(screen.getByLabelText('Old password'), { target: { value: 'OldPassword123!' } });
        fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
        fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '654321' } });
        fireEvent.click(screen.getByText('Save change'));

        await waitFor(() => {
            expect(patchMock).toHaveBeenCalledTimes(1);
            expect(reauthMock.mock.calls).toEqual([[{ purpose: 'password_set', current_password: 'OldPassword123!', otp: '654321' }]]);
            expect(patchMock.mock.calls[0][0]).not.toHaveProperty('otp');
        });
    });

    describe('account without a password (passkey only)', () => {
        beforeEach(() => {
            beginPasswordSetReauthMock.mockClear();
            setAccountState({ account_id: 'abc', auth_methods: ['passkey'], mfa: { totp: { enabled: true } } });
            useAppStore.getState().setPasskeys([{ id: 'p1' }]);
        });

        it('blocks submission until the passkey is verified', async () => {
            setAccountState({ account_id: 'abc', auth_methods: ['passkey'], mfa: { totp: { enabled: false } } });
            render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

            expect(screen.queryByLabelText('Old password')).toBeNull();
            fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
            fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
            fireEvent.click(screen.getByText('Save change'));

            await waitFor(() => {
                expect(patchMock).not.toHaveBeenCalled();
            });
        });

        it('sends the passkey reauth token as the password test operation, without a 2FA code', async () => {
            render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

            fireEvent.click(screen.getByText('Verify with passkey'));
            await waitFor(() => expect(screen.getByText('Passkey verified')).toBeTruthy());
            expect(screen.queryByLabelText('2FA code')).toBeNull();
            fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
            fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
            fireEvent.click(screen.getByText('Save change'));

            await waitFor(() => {
                expect(beginPasswordSetReauthMock).toHaveBeenCalledTimes(1);
                expect(reauthMock).not.toHaveBeenCalled();
                expect(patchMock).toHaveBeenCalledTimes(1);
                const call = patchMock.mock.calls[0];
                expect(call[0].updates).toEqual([
                    { operation: 'test', path: '/password', value: { reauth_token: 'reauth-token' } },
                    { operation: 'replace', path: '/password', value: 'NewPassword123!' },
                ]);
                expect(call).toHaveLength(1);
            });
        });
    });
});
