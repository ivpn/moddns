import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useIdentityVerification } from '@/hooks/useIdentityVerification';

vi.mock('@/api/api', () => ({
    default: { Client: { accountsApi: { apiV1AccountsReauthPost: vi.fn() } } },
}));

import api from '@/api/api';

const reauthPost = vi.mocked(api.Client.accountsApi.apiV1AccountsReauthPost);

const refusal = (status: number, error: string) =>
    Object.assign(new Error('Request failed'), { isAxiosError: true, response: { status, data: { error } } });

function setup(verification: { currentPassword?: string; otp?: string; reauthToken?: string }) {
    const hook = renderHook(() => useIdentityVerification());
    act(() => hook.result.current.onChange(verification));
    return hook;
}

// specRef: api-endpoint-behaviour B15
describe('useIdentityVerification - reauth token', () => {
    beforeEach(() => {
        let issued = 0;
        reauthPost.mockReset().mockImplementation(() =>
            Promise.resolve({ data: { reauth_token: `issued-${++issued}`, expires_at: new Date(Date.now() + 300_000).toISOString() } } as never));
    });

    it('passes a passkey token through without a password exchange', async () => {
        const { result } = setup({ reauthToken: 'passkey-token', otp: '123456' });
        await expect(result.current.reauthToken('totp_disable')).resolves.toBe('passkey-token');
        expect(reauthPost).not.toHaveBeenCalled();
    });

    it('exchanges the password and code, and sends no code for an operation that checks it itself', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await expect(result.current.reauthToken('email_change')).resolves.toBe('issued-1');
        await expect(result.current.reauthToken('totp_disable')).resolves.toBe('issued-2');
        expect(reauthPost.mock.calls).toEqual([
            [{ purpose: 'email_change', current_password: 'pw', otp: '123456' }],
            [{ purpose: 'totp_disable', current_password: 'pw' }],
        ]);
    });

    it('a new request after a success gets a new token', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await result.current.reauthToken('account_deletion');
        await expect(result.current.reauthToken('account_deletion')).resolves.toBe('issued-2');
    });

    it('a retry after a failure that left the token unused reuses it once', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await result.current.reauthToken('account_deletion');
        act(() => result.current.requestFailed(refusal(400, 'invalid deletion code')));
        await expect(result.current.reauthToken('account_deletion')).resolves.toBe('issued-1');
        await expect(result.current.reauthToken('account_deletion')).resolves.toBe('issued-2');
        expect(reauthPost).toHaveBeenCalledTimes(2);
    });

    it('a token of an operation that uses it up first is not reused after a failure', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await result.current.reauthToken('passkey_add');
        act(() => result.current.requestFailed(new Error('Passkey addition was cancelled or timed out.')));
        await expect(result.current.reauthToken('passkey_add')).resolves.toBe('issued-2');
    });

    it('a token the server refused is not reused', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await result.current.reauthToken('account_deletion');
        act(() => result.current.requestFailed(refusal(400, 'invalid reauth token')));
        await expect(result.current.reauthToken('account_deletion')).resolves.toBe('issued-2');
    });

    it('a token is not reused for another purpose, password or code', async () => {
        const hook = setup({ currentPassword: 'pw', otp: '123456' });
        await hook.result.current.reauthToken('email_change');
        act(() => hook.result.current.requestFailed(refusal(400, 'new email address is the same as the current one')));
        act(() => hook.result.current.onChange({ currentPassword: 'pw', otp: '654321' }));
        await expect(hook.result.current.reauthToken('email_change')).resolves.toBe('issued-2');
    });

    it('a refused password exchange leaves nothing to reuse', async () => {
        const { result } = setup({ currentPassword: 'pw', otp: '123456' });
        await result.current.reauthToken('sessions_delete');
        reauthPost.mockRejectedValueOnce(refusal(400, 'invalid current password'));
        await expect(result.current.reauthToken('sessions_delete')).rejects.toThrow();
        act(() => result.current.requestFailed(refusal(400, 'invalid current password')));
        await expect(result.current.reauthToken('sessions_delete')).resolves.toBe('issued-2');
    });

    it('closing the fields drops a reusable token', async () => {
        const hook = setup({ currentPassword: 'pw' });
        await hook.result.current.reauthToken('password_set');
        act(() => hook.result.current.requestFailed(new Error('Network Error')));
        act(() => hook.result.current.onChange(null));
        act(() => hook.result.current.onChange({ currentPassword: 'pw' }));
        await expect(hook.result.current.reauthToken('password_set')).resolves.toBe('issued-2');
    });

    it('refuses without a verification', async () => {
        const { result } = renderHook(() => useIdentityVerification());
        await expect(result.current.reauthToken('profile_export')).rejects.toThrow('Verify your identity first.');
    });
});
