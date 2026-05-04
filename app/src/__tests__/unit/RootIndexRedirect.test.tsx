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

// Stub the lazy-loaded Landing component so the unauth case renders synchronously.
vi.mock('@/pages/landing/Landing', () => ({
    default: () => <div data-testid="landing-page" />,
}));

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

    it('renders the landing page when auth state is false', async () => {
        localStorage.setItem(AUTH_KEY, 'true');

        renderWithAuth({ isAuthenticated: false });

        expect(await screen.findByTestId('landing-page')).toBeInTheDocument();
        expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    });

    it('renders the landing page when local storage flag is missing', async () => {
        localStorage.removeItem(AUTH_KEY);

        renderWithAuth({ isAuthenticated: true });

        expect(await screen.findByTestId('landing-page')).toBeInTheDocument();
        expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    });
});
