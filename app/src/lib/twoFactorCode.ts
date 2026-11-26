/** Which kind of 2FA code a field takes: a code from the authenticator app or a backup code. */
export type TwoFactorCodeMode = 'authenticator' | 'backup';

// The API ignores hyphens and whitespace in a 2FA code (api-endpoint-behaviour E6),
// so a code is checked here in the same normalised form.
export const normalizeTwoFactorCode = (value: string): string => value.replace(/[\s-]/g, '');

const AUTHENTICATOR_CODE = /^\d{6}$/;
// Current backup codes: 25 characters of the API's AlphaNumericUserFriendlyUppercase
// alphabet, accepted in any case (E5, E6).
const BACKUP_CODE = /^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{25}$/i;
// Backup codes of earlier formats: 8, 16 or 20 mixed-case letters and digits (E8).
const LEGACY_BACKUP_CODE = /^(?:[A-Za-z0-9]{8}|[A-Za-z0-9]{16}|[A-Za-z0-9]{20})$/;

/** True when value has the format of the chosen kind of code, so a typo is caught before it costs a lockout attempt (E12). */
export function isTwoFactorCodeFormatValid(value: string, mode: TwoFactorCodeMode): boolean {
    const code = normalizeTwoFactorCode(value);
    if (mode === 'authenticator') return AUTHENTICATOR_CODE.test(code);
    return BACKUP_CODE.test(code) || LEGACY_BACKUP_CODE.test(code);
}

/** Total backup codes issued per set (E2, E13). */
export const BACKUP_CODE_SET_SIZE = 8;
/** At or below this many unused codes the app suggests a new set. */
export const BACKUP_CODES_LOW = 2;
