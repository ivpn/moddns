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

// Stub the lazy-loaded Landing component so it renders synchronously and
// surfaces its `isAuthenticated` prop in the DOM for assertion.
vi.mock('@/pages/landing/Landing', () => ({
    default: ({ isAuthenticated }: { isAuthenticated?: boolean }) => (
        <div data-testid="landing-page" data-authed={String(Boolean(isAuthenticated))} />
    ),
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

    it('renders the landing page with isAuthenticated=true when both auth state and local storage agree', async () => {
        localStorage.setItem(AUTH_KEY, 'true');

        renderWithAuth({ isAuthenticated: true });

        const landing = await screen.findByTestId('landing-page');
        expect(landing).toBeInTheDocument();
        expect(landing).toHaveAttribute('data-authed', 'true');
        expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    });

    it('renders the landing page with isAuthenticated=false when auth state is false', async () => {
        localStorage.setItem(AUTH_KEY, 'true');

        renderWithAuth({ isAuthenticated: false });

        const landing = await screen.findByTestId('landing-page');
        expect(landing).toBeInTheDocument();
        expect(landing).toHaveAttribute('data-authed', 'false');
        expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    });

    it('renders the landing page with isAuthenticated=false when local storage flag is missing', async () => {
        localStorage.removeItem(AUTH_KEY);

        renderWithAuth({ isAuthenticated: true });

        const landing = await screen.findByTestId('landing-page');
        expect(landing).toBeInTheDocument();
        // Belt-and-braces guard: stale React state without localStorage backing
        // does not flip the page into the authenticated UI.
        expect(landing).toHaveAttribute('data-authed', 'false');
        expect(screen.queryByTestId('navigate')).not.toBeInTheDocument();
    });
});
