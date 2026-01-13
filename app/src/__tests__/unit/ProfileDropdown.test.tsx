import { render, screen, cleanup, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom';
import ProfileDropdown from '@/pages/header/ProfileDropdown';
import { MemoryRouter } from 'react-router-dom';
import { describe, test, expect, beforeAll } from 'vitest';

// Minimal mocks for Select components (assuming shadcn Select is headless and fine to render)

describe('ProfileDropdown truncation', () => {
    // Base mock; individual tests override as needed
    beforeAll(() => {
        if (!(window as any).matchMedia) {
            (window as any).matchMedia = (q: string) => ({
                matches: false,
                media: q,
                addEventListener: () => { },
                removeEventListener: () => { }
            });
        }
        // Radix Select reads PointerEvent APIs that jsdom does not implement
        if (!Element.prototype.hasPointerCapture) {
            Element.prototype.hasPointerCapture = () => false;
        }
        if (!Element.prototype.releasePointerCapture) {
            Element.prototype.releasePointerCapture = () => { };
        }
        if (!Element.prototype.scrollIntoView) {
            Element.prototype.scrollIntoView = () => { };
        }
    });
    const baseProfile = { profile_id: 'id1', name: 'ShortName' } as any;
    const longProfile = { profile_id: 'id2', name: 'VeryLongProfileNameExceedingLimit' } as any;

    const noop = () => { };

    function setup(current: any) {
        return render(
            <MemoryRouter>
                <ProfileDropdown
                    profiles={[baseProfile, longProfile]}
                    currentProfile={current}
                    setActiveProfile={noop as any}
                    setProfiles={noop as any}
                />
            </MemoryRouter>
        );
    }

    test('shows full name when length <= 16', () => {
        setup(baseProfile);
        expect(screen.getByTestId('profile-name-full')).toHaveTextContent('ShortName');
        expect(screen.queryByTestId('profile-name-truncated')).toBeNull();
    });

    test('mobile (<768px) uses 16-char truncation', () => {
        // Mock mobile viewport
        (window.matchMedia as any) = (q: string) => ({ matches: false, media: q, addEventListener: () => { }, removeEventListener: () => { } });
        setup(longProfile);
        const truncated = screen.getByTestId('profile-name-truncated');
        expect(truncated).toHaveTextContent(longProfile.name.slice(0, 16) + '…');
    });

    test('tablet/desktop (>=768px) uses 20-char truncation', () => {
        cleanup();
        (window.matchMedia as any) = (q: string) => ({ matches: q === '(min-width: 768px)', media: q, addEventListener: () => { }, removeEventListener: () => { } });
        setup(longProfile);
        const truncated = screen.getByTestId('profile-name-truncated');
        expect(truncated).toHaveTextContent(longProfile.name.slice(0, 20) + '…');
    });

    test('select closes when opening Create Profile dialog', async () => {
        (window.matchMedia as any) = (q: string) => ({ matches: q === '(min-width: 768px)', media: q, addEventListener: () => { }, removeEventListener: () => { } });
        const user = userEvent.setup();
        render(
            <MemoryRouter>
                <ProfileDropdown
                    profiles={[baseProfile, longProfile]}
                    currentProfile={baseProfile}
                    setActiveProfile={noop as any}
                    setProfiles={noop as any}
                />
            </MemoryRouter>
        );

        const trigger = screen.getByRole('combobox');
        await user.click(trigger);
        await user.click(screen.getByText('Create profile'));

        await waitFor(() => {
            expect(document.querySelector('[data-slot="select-content"]')).not.toBeInTheDocument();
        });
        expect(screen.getByPlaceholderText('Type a name')).toBeInTheDocument();
    });

    test('select closes when opening Edit Profile dialog', async () => {
        (window.matchMedia as any) = (q: string) => ({ matches: q === '(min-width: 768px)', media: q, addEventListener: () => { }, removeEventListener: () => { } });
        const user = userEvent.setup();
        render(
            <MemoryRouter>
                <ProfileDropdown
                    profiles={[baseProfile, longProfile]}
                    currentProfile={baseProfile}
                    setActiveProfile={noop as any}
                    setProfiles={noop as any}
                />
            </MemoryRouter>
        );

        const trigger = screen.getByRole('combobox');
        await user.click(trigger);
        // Click settings icon for selected profile
        const editButton = await screen.findByTestId('edit-profile-settings');
        await user.click(editButton);

        await waitFor(() => {
            expect(document.querySelector('[data-slot="select-content"]')).not.toBeInTheDocument();
        });
        expect(screen.getByText('Edit profile')).toBeInTheDocument();
    });
});
