// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { toast } from 'sonner';
import { AuthContext } from '@/App';
import { useLogout } from '@/hooks/useLogout';
import api from '@/api/api';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

vi.mock('@/api/api', () => ({
    default: { Client: { authApi: { apiV1AccountsLogoutPost: vi.fn() } } },
}));

const logoutPost = api.Client.authApi.apiV1AccountsLogoutPost as unknown as ReturnType<typeof vi.fn>;

function setup(appLogout: () => void) {
    const wrapper = ({ children }: { children: ReactNode }) => (
        <AuthContext.Provider value={{ isAuthenticated: true, login: vi.fn(), logout: appLogout }}>
            {children}
        </AuthContext.Provider>
    );
    return renderHook(() => useLogout(), { wrapper });
}

describe('useLogout', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    // specRef: A2a
    it('clears app state only after the API has ended the session', async () => {
        const order: string[] = [];
        logoutPost.mockImplementation(async () => { order.push('api'); return {}; });
        const appLogout = vi.fn(() => { order.push('app'); });
        const { result } = setup(appLogout);

        let ended: boolean | undefined;
        await act(async () => { ended = await result.current.logout(); });

        expect(ended).toBe(true);
        expect(order).toEqual(['api', 'app']);
        expect(toast.error).not.toHaveBeenCalled();
        expect(result.current.loading).toBe(false);
    });

    // specRef: A2a
    it('keeps the user logged in and reports the failure when the call fails', async () => {
        logoutPost.mockRejectedValue({ response: { status: 503 } });
        const appLogout = vi.fn();
        const { result } = setup(appLogout);

        let ended: boolean | undefined;
        await act(async () => { ended = await result.current.logout(); });

        expect(ended).toBe(false);
        expect(appLogout).not.toHaveBeenCalled();
        expect(toast.error).toHaveBeenCalledWith('Logout failed.');
    });

    // specRef: A2a
    it('clears app state without a failure toast when the session had already ended', async () => {
        logoutPost.mockRejectedValue({ response: { status: 401 } });
        const appLogout = vi.fn();
        const { result } = setup(appLogout);

        let ended: boolean | undefined;
        await act(async () => { ended = await result.current.logout(); });

        expect(ended).toBe(true);
        expect(toast.error).not.toHaveBeenCalled();
        expect(appLogout).toHaveBeenCalledTimes(1);
    });
});
