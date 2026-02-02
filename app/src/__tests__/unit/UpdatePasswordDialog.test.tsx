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
const currentGetMock = vi.fn().mockResolvedValue({ data: { account_id: 'abc', mfa: { totp: { enabled: false } } } });
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            accountsApi: {
                apiV1AccountsPatch: (...args: unknown[]) => patchMock(...args),
                apiV1AccountsCurrentGet: () => currentGetMock()
            }
        }
    }
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

    it('submits test + replace operations in correct order', async () => {
        render(<UpdatePasswordDialog open={true} onOpenChange={() => { }} />);

        fireEvent.change(screen.getByLabelText('Old password'), { target: { value: 'OldPassword123!' } });
        fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'NewPassword123!' } });
        fireEvent.change(screen.getByLabelText('Confirm password'), { target: { value: 'NewPassword123!' } });
        fireEvent.click(screen.getByText('Save change'));

        await waitFor(() => {
            expect(patchMock).toHaveBeenCalledTimes(1);
            const [arg] = patchMock.mock.calls[0];
            expect(arg.updates).toHaveLength(2);
            expect(arg.updates[0].operation).toBe('test');
            expect(arg.updates[0].path).toBe('/password');
            expect(arg.updates[1].operation).toBe('replace');
            expect(arg.updates[1].path).toBe('/password');
        });
    });

    it('requires OTP when 2FA enabled and passes it', async () => {
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
            const call = patchMock.mock.calls[0];
            // With 2FA enabled, patch is called with payload, otp, providers array
            expect(call[1]).toBe('654321');
            expect(Array.isArray(call[2])).toBe(true);
        });
    });
});
