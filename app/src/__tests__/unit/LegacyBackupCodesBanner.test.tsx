import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { useAppStore } from '@/store/general';
import LegacyBackupCodesBanner from '@/components/LegacyBackupCodesBanner';

const setTotp = (totp: Record<string, unknown>) =>
    useAppStore.getState().setAccount({ id: 'a1', mfa: { totp } } as never);

const renderBanner = () => render(<MemoryRouter><LegacyBackupCodesBanner /></MemoryRouter>);

// specRef: api-endpoint-behaviour E14
describe('LegacyBackupCodesBanner', () => {
    beforeEach(() => {
        useAppStore.getState().resetAccountState();
        localStorage.clear();
    });

    it('advises regenerating backup codes of an earlier format', () => {
        setTotp({ enabled: true, backup_codes_legacy: true, backup_codes_remaining: 8 });
        renderBanner();
        const banner = screen.getByTestId('legacy-backup-codes-banner');
        expect(banner).toHaveTextContent('We strongly advise regenerating your 2FA backup codes.');
        expect(banner).toHaveTextContent('They still work');
        expect(screen.getByRole('button', { name: 'Regenerate backup codes' })).toBeInTheDocument();
    });

    it.each([
        ['current codes', { enabled: true, backup_codes_legacy: false, backup_codes_remaining: 8 }],
        ['2FA off', { enabled: false, backup_codes_legacy: true }],
        ['account not loaded', undefined],
    ])('shows nothing for %s', (_, totp) => {
        if (totp) setTotp(totp);
        renderBanner();
        expect(screen.queryByTestId('legacy-backup-codes-banner')).toBeNull();
    });

    it('stays closed until the account state is reset at sign-out', () => {
        setTotp({ enabled: true, backup_codes_legacy: true });
        const { unmount } = renderBanner();
        fireEvent.click(screen.getByTestId('legacy-backup-codes-banner-close'));
        expect(screen.queryByTestId('legacy-backup-codes-banner')).toBeNull();
        unmount();

        const again = renderBanner();
        expect(screen.queryByTestId('legacy-backup-codes-banner')).toBeNull();
        again.unmount();

        useAppStore.getState().resetAccountState();
        setTotp({ enabled: true, backup_codes_legacy: true });
        renderBanner();
        expect(screen.getByTestId('legacy-backup-codes-banner')).toBeInTheDocument();
    });

    it('opens the regenerate dialog', () => {
        setTotp({ enabled: true, backup_codes_legacy: true });
        renderBanner();
        fireEvent.click(screen.getByTestId('legacy-backup-codes-banner-regenerate'));
        expect(screen.getByTestId('regenerate-backup-codes-dialog')).toBeInTheDocument();
        expect(screen.getByLabelText('2FA code')).toBeInTheDocument();
    });
});
