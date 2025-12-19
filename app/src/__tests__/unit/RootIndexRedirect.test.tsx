import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AuthContext, RootIndexRedirect } from '@/App';
import { AUTH_KEY } from '@/lib/consts';

vi.mock('react-router-dom', async () => {
    const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
    return {
        ...actual,
        Navigate: ({ to }: { to: string }) => <div data-testid="navigate" data-to={to} />,
    };
});

describe('RootIndexRedirect', () => {
    type AuthContextValue = React.ContextType<typeof AuthContext>;

    const renderWithAuth = (value: Partial<AuthContextValue>) => {
        const ctx: AuthContextValue = {
            isAuthenticated: false,
            login: vi.fn(),
            logout: vi.fn(),
            ...value,
        };

        return render(
            <AuthContext.Provider value={ctx}>
                <RootIndexRedirect />
            </AuthContext.Provider>
        );
    };

    beforeEach(() => {
        localStorage.clear();
    });

    it('navigates to /home when both auth state and local storage are true', () => {
        localStorage.setItem(AUTH_KEY, 'true');

        renderWithAuth({ isAuthenticated: true });

        expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/home');
    });

    it('navigates to /login when auth state is false', () => {
        localStorage.setItem(AUTH_KEY, 'true');

        renderWithAuth({ isAuthenticated: false });

        expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/login');
    });

    it('falls back to /login when local storage flag is missing', () => {
        localStorage.removeItem(AUTH_KEY);

        renderWithAuth({ isAuthenticated: true });

        expect(screen.getByTestId('navigate')).toHaveAttribute('data-to', '/login');
    });
});
