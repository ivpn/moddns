// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { toast } from 'sonner';
import { AuthContext } from '@/App';
import NavigationMenu from '@/pages/navigation_menu/NavigationMenu';
import { NavigationCollapseProvider } from '@/context/NavigationCollapseContext';
import api from '@/api/api';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            announcementsApi: { apiV1AnnouncementsGet: vi.fn().mockResolvedValue({ data: [] }) },
            authApi: { apiV1AccountsLogoutPost: vi.fn() },
        },
    },
}));

const logoutPost = api.Client.authApi.apiV1AccountsLogoutPost as unknown as ReturnType<typeof vi.fn>;

function renderMenu(logout: () => void) {
    render(
        <AuthContext.Provider value={{ isAuthenticated: true, login: vi.fn(), logout }}>
            <MemoryRouter initialEntries={['/account-preferences']}>
                <NavigationCollapseProvider>
                    <NavigationMenu />
                </NavigationCollapseProvider>
            </MemoryRouter>
        </AuthContext.Provider>,
    );
    fireEvent.click(screen.getByTestId('btn-nav-logout'));
    fireEvent.click(screen.getByTestId('btn-confirm-logout'));
}

describe('navigation menu logout', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    // specRef: A2a
    it('logs out in the app only after the API has ended the session', async () => {
        const order: string[] = [];
        logoutPost.mockImplementation(async () => { order.push('api'); return {}; });
        const logout = vi.fn(() => { order.push('app'); });

        renderMenu(logout);

        await waitFor(() => expect(logout).toHaveBeenCalledTimes(1));
        expect(order).toEqual(['api', 'app']);
    });

    // specRef: A2a
    it('stays logged in and keeps the confirm dialog open when the logout call fails', async () => {
        logoutPost.mockRejectedValue(new Error('network'));
        const logout = vi.fn();

        renderMenu(logout);

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Logout failed.'));
        expect(logout).not.toHaveBeenCalled();
        expect(screen.getByTestId('btn-confirm-logout')).not.toBeDisabled();
    });
});
