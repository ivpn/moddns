import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, test, expect } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import LoginCard from '@/pages/auth/LoginCard';

function renderLoginCard() {
    return render(
        <MemoryRouter initialEntries={[{ pathname: '/login' }]}>
            <LoginCard />
        </MemoryRouter>
    );
}

describe('LoginCard logo link', () => {
    test('logo is wrapped in a link pointing to the landing page', () => {
        renderLoginCard();
        const link = screen.getByRole('link', { name: /modDNS home/i });
        expect(link).toHaveAttribute('href', '/');
        expect(link.querySelector('img[alt="modDNS logo"]')).not.toBeNull();
    });
});
