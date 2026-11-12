// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DeleteAccountDialog from '@/pages/account_preferences/DeleteAccount';
import { AuthContext } from '@/App';
import { useAppStore } from '@/store/general';
import api from '@/api/api';
import type { ModelAccount, ResponsesPasskey } from '@/api/client/api';
import { beginAccountDeletionReauth } from '@/lib/webauthn';
import type React from 'react';

vi.mock('sonner', () => ({
    toast: {
        success: vi.fn(),
        error: vi.fn(),
    },
}));

vi.mock('@/api/api', () => {
    return {
        default: {
            Client: {
                accountsApi: {
                    apiV1AccountsCurrentDeletionCodePost: vi.fn(),
                    apiV1AccountsCurrentDelete: vi.fn(),
                },
            },
        },
    };
});

vi.mock('@/lib/webauthn', () => ({
    beginAccountDeletionReauth: vi.fn(),
}));

type GenericMock = ReturnType<typeof vi.fn>;

const deletionCodeMock = api.Client.accountsApi.apiV1AccountsCurrentDeletionCodePost as unknown as GenericMock;
const deleteAccountMock = api.Client.accountsApi.apiV1AccountsCurrentDelete as unknown as GenericMock;

const mockedBeginReauth = beginAccountDeletionReauth as unknown as GenericMock;

const renderDialog = (overrides?: Partial<React.ComponentProps<typeof DeleteAccountDialog>>) => {
    const logout = vi.fn();
    const value = { isAuthenticated: true, login: vi.fn(), logout };

    const props: React.ComponentProps<typeof DeleteAccountDialog> = {
        open: true,
        onOpenChange: vi.fn(),
        loading: false,
        ...overrides,
    };

    render(
        <AuthContext.Provider value={value}>
            <DeleteAccountDialog {...props} />
        </AuthContext.Provider>
    );

    return { logout, props };
};

describe('DeleteAccountDialog', () => {
    beforeEach(() => {
        deletionCodeMock.mockReset();
        deleteAccountMock.mockReset();
        mockedBeginReauth.mockReset();
        useAppStore.setState({ account: null, passkeys: [] });
    });

    it('requires password + OTP before confirming deletion and forwards headers', async () => {
        const user = userEvent.setup();

        const account = {
            id: 'acct-1',
            auth_methods: ['password'],
            mfa: { totp: { enabled: true } },
        } as unknown as ModelAccount;
        useAppStore.setState({ account, passkeys: [] });

        deletionCodeMock.mockResolvedValue({ data: { code: 'DELETE123' } });
        deleteAccountMock.mockResolvedValue({});

        const { logout } = renderDialog();

        await waitFor(() => expect(deletionCodeMock).toHaveBeenCalledTimes(1));
        const [generationOptions] = deletionCodeMock.mock.calls[0];
        expect(generationOptions).toEqual({});

        await screen.findByDisplayValue('DELETE123');

        await user.type(screen.getByLabelText('Current password'), 'Password123!');
        await user.type(screen.getByLabelText('2FA code'), '123456');

        await user.clear(screen.getByPlaceholderText('Enter the deletion code'));
        await user.type(screen.getByPlaceholderText('Enter the deletion code'), 'DELETE123');

        await user.click(screen.getByText('Delete account'));

        await waitFor(() => expect(deleteAccountMock).toHaveBeenCalledTimes(1));
        const [payload, options] = deleteAccountMock.mock.calls[0];
        expect(payload).toMatchObject({
            deletion_code: 'DELETE123',
            current_password: 'Password123!',
        });
        expect(options?.headers).toMatchObject({ 'x-mfa-code': '123456', 'x-mfa-methods': 'totp' });
        expect(logout).toHaveBeenCalledWith('Account deleted.', 'info');
    });

    it('uses passkey reauthentication token for deletion flow', async () => {
        const account = {
            id: 'acct-2',
            auth_methods: ['passkey'],
            mfa: { totp: { enabled: false } },
        } as unknown as ModelAccount;
        const credential = { id: 'cred-1' } as unknown as ResponsesPasskey;
        useAppStore.setState({ account, passkeys: [credential] });

        mockedBeginReauth.mockResolvedValue('reauth-token-1');
        deletionCodeMock.mockResolvedValue({ data: { code: 'DEL-9999' } });
        deleteAccountMock.mockResolvedValue({});

        const { logout } = renderDialog();

        const user = userEvent.setup();

        await waitFor(() => expect(deletionCodeMock).toHaveBeenCalledTimes(1));
        const [generationOptions] = deletionCodeMock.mock.calls[0];
        expect(generationOptions).toEqual({});

        await screen.findByDisplayValue('DEL-9999');

        const verifyButton = screen.getByText('Verify with passkey').closest('button');

        await user.click(verifyButton!);
        await waitFor(() => expect(mockedBeginReauth).toHaveBeenCalledTimes(1));
        await screen.findByText('Passkey verified');

        await user.clear(screen.getByPlaceholderText('Enter the deletion code'));
        await user.type(screen.getByPlaceholderText('Enter the deletion code'), 'DEL-9999');
        await user.click(screen.getByText('Delete account'));

        await waitFor(() => expect(deleteAccountMock).toHaveBeenCalledTimes(1));
        const [payload, options] = deleteAccountMock.mock.calls[0];
        expect(payload).toMatchObject({
            deletion_code: 'DEL-9999',
            reauth_token: 'reauth-token-1',
        });
        expect(options?.headers ?? {}).not.toHaveProperty('x-mfa-code');
        expect(logout).toHaveBeenCalledWith('Account deleted.', 'info');
    });
});
