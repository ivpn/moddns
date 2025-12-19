import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import Login from '@/pages/auth/Login';
import { AuthContext } from '@/App';
import api from '@/api/api';

const navigateMock = vi.fn();

vi.mock('react-router-dom', async () => {
    const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
    return {
        ...actual,
        useNavigate: () => navigateMock,
    };
});

vi.mock('@/pages/auth/LoginCard', () => ({
    default: () => <div data-testid="login-card">login-card</div>,
}));

vi.mock('@/components/auth/AuthFooter', () => ({
    default: () => <div data-testid="auth-footer" />,
}));

vi.mock('@/components/dialogs/SessionLimitDialog', () => ({
    default: () => null,
}));

const revalidateSpy = vi.spyOn(api.Client.accountsApi, 'apiV1AccountsCurrentGet');

type AuthContextValue = React.ContextType<typeof AuthContext>;

function renderLogin(isAuthenticated: boolean, overrides: Partial<AuthContextValue> = {}) {
    const value: AuthContextValue = {
        isAuthenticated,
        login: vi.fn(),
        logout: vi.fn(),
        ...overrides,
    };

    return render(
        <MemoryRouter initialEntries={[{ pathname: '/login' }]}>
            <AuthContext.Provider value={value}>
                <Login />
            </AuthContext.Provider>
        </MemoryRouter>
    );
}

describe('Login page behavior without legacy revalidation screen', () => {
    beforeEach(() => {
        navigateMock.mockReset();
        revalidateSpy.mockReset();
    });

    it('renders the login card immediately even when auth context is true', () => {
        renderLogin(true);

        expect(screen.getByTestId('login-card')).toBeInTheDocument();
        expect(screen.queryByTestId('loading-screen')).not.toBeInTheDocument();
        expect(revalidateSpy).not.toHaveBeenCalled();
        expect(navigateMock).not.toHaveBeenCalled();
    });

    it('renders the login card when auth state is false without performing revalidation', () => {
        renderLogin(false);

        expect(screen.getByTestId('login-card')).toBeInTheDocument();
        expect(screen.queryByTestId('loading-screen')).not.toBeInTheDocument();
        expect(revalidateSpy).not.toHaveBeenCalled();
        expect(navigateMock).not.toHaveBeenCalled();
    });
});
