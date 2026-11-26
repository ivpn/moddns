import { useState } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, expect, it, vi } from 'vitest';
import TwoFactorCodeInput from '@/components/TwoFactorCodeInput';
import { isTwoFactorCodeFormatValid } from '@/lib/twoFactorCode';

// specRef: api-endpoint-behaviour E6
describe('isTwoFactorCodeFormatValid', () => {
    it.each([
        ['123456', true],
        ['123 456', true],
        ['123-456', true],
        ['12345', false],
        ['1234567', false],
        ['12345a', false],
        ['GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5', false],
    ])('authenticator %s -> %s', (code, valid) => {
        expect(isTwoFactorCodeFormatValid(code, 'authenticator')).toBe(valid);
    });

    it.each([
        ['GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5', true],
        ['gfuks z7kzb vpndn tmwms hakr5', true],
        ['GFUKSZ7KZBVPNDNTMWMSHAKR5', true],
        ['LegacyCodeAAAAA1', true], // 16 mixed-case characters (E8)
        ['Abc2Def3', true], // 8 characters (E8)
        ['AbCdEfGh2345JkMnPqRs', true], // 20 characters (E8)
        ['GFUKS-Z7KZB-VPNDN-TMWMS-HAKR', false], // 24 characters
        ['GFUKS-Z7KZB-VPNDN', false], // 15 characters: part of a current code
        ['Abc2Def3G', false], // 9 characters: no format has this length
        ['GFUKS-Z7KZB-VPNDN-TMWMS-HAKR1', false], // 1 is not in the code alphabet
        ['Abc2Def', false],
        ['123456', false],
        ['Abc2Def3!', false],
    ])('backup %s -> %s', (code, valid) => {
        expect(isTwoFactorCodeFormatValid(code, 'backup')).toBe(valid);
    });
});

function Harness({ onChange = vi.fn(), allowBackupCode = true }: { onChange?: (v: string, ok: boolean) => void; allowBackupCode?: boolean }) {
    const [value, setValue] = useState('');
    return (
        <TwoFactorCodeInput
            id="code"
            data-testid="code"
            value={value}
            allowBackupCode={allowBackupCode}
            onChange={(v, ok) => { setValue(v); onChange(v, ok); }}
        />
    );
}

// specRef: api-endpoint-behaviour E6, E13
describe('TwoFactorCodeInput', () => {
    it('asks for an authenticator code on a numeric keypad by default', () => {
        render(<Harness />);
        const input = screen.getByLabelText('2FA code');
        expect(input).toHaveAttribute('inputmode', 'numeric');
        expect(input).toHaveAttribute('autocomplete', 'one-time-code');
        expect(screen.getByText('6-digit code from your authenticator app')).toBeInTheDocument();
    });

    it('switches to a backup code on a text keyboard and clears the field', () => {
        const onChange = vi.fn();
        render(<Harness onChange={onChange} />);
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '123' } });

        fireEvent.click(screen.getByRole('button', { name: 'Lost your device? Use a backup code' }));

        const input = screen.getByLabelText('Backup code');
        expect(input).toHaveAttribute('inputmode', 'text');
        expect(input).toHaveAttribute('autocapitalize', 'none');
        expect(input).toHaveValue('');
        expect(onChange).toHaveBeenLastCalledWith('', false);

        fireEvent.change(input, { target: { value: 'GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5' } });
        expect(onChange).toHaveBeenLastCalledWith('GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5', true);

        fireEvent.click(screen.getByRole('button', { name: 'Use your authenticator app instead' }));
        expect(screen.getByLabelText('2FA code')).toHaveAttribute('inputmode', 'numeric');
    });

    it('reports validity for the chosen mode only', () => {
        const onChange = vi.fn();
        render(<Harness onChange={onChange} />);
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '1234567' } });
        expect(onChange).toHaveBeenLastCalledWith('1234567', false);
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '123 456' } });
        expect(onChange).toHaveBeenLastCalledWith('123 456', true);
    });

    it('explains a malformed code once the field is left', () => {
        render(<Harness />);
        const input = screen.getByLabelText('2FA code');
        fireEvent.change(input, { target: { value: '12345' } });
        expect(input).not.toHaveAttribute('aria-invalid');
        fireEvent.blur(input);
        expect(input).toHaveAttribute('aria-invalid', 'true');
        expect(screen.getByText('Enter the 6-digit code from your authenticator app.')).toBeInTheDocument();
    });

    it('switches to the backup-code field when a backup code is pasted', () => {
        const onChange = vi.fn();
        render(<Harness onChange={onChange} />);
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: 'GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5' } });
        expect(onChange).toHaveBeenLastCalledWith('GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5', true);
        expect(screen.getByLabelText('Backup code')).toHaveValue('GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5');
    });

    it('stays on the authenticator field for a mistyped all-digit code', () => {
        render(<Harness />);
        fireEvent.change(screen.getByLabelText('2FA code'), { target: { value: '12345678' } });
        expect(screen.getByLabelText('2FA code')).toHaveAttribute('inputmode', 'numeric');
    });

    it('offers no backup code where only an authenticator code is accepted', () => {
        render(<Harness allowBackupCode={false} />);
        expect(screen.queryByRole('button', { name: /backup code/ })).toBeNull();
    });
});
