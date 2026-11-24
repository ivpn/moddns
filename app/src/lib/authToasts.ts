import { createElement } from 'react';
import { toast } from 'sonner';

export const TOTP_TOO_MANY_ATTEMPTS_MESSAGE = 'Too many wrong 2FA codes. Please try again later.';

/** True when the API refused a 2FA code because the account is locked after repeated failures. */
export const isTotpLockoutError = (apiErr?: string): boolean =>
    !!apiErr?.startsWith('too many failed 2FA attempts');

/** The message for a refused 2FA code: the lockout in plain words, otherwise the API's own text. */
export const totpErrorMessage = (apiErr: string | undefined, fallback: string): string =>
    isTotpLockoutError(apiErr) ? TOTP_TOO_MANY_ATTEMPTS_MESSAGE : (apiErr || fallback);

export const AUTH_TOAST_IDS = {
    loginSuccess: 'toast-login-success',
    logoutSuccess: 'toast-logout-success',
    sessionExpired: 'toast-session-expired',
    loginInvalid: 'toast-login-invalid',
    loginUnauthorized: 'toast-login-unauthorized',
    loginTooManyAttempts: 'toast-login-rate-limit',
    loginTotpTooManyAttempts: 'toast-login-totp-rate-limit',
    loginUnexpectedError: 'toast-login-error',
    loginTOTPRequired: 'toast-login-totp-required',
    passkeyError: 'toast-passkey-error',
    passwordResetSuccess: 'toast-password-reset-success',
    accountDeletedSuccess: 'toast-account-deleted-success',
    accountCreatedSuccess: 'toast-account-created-success',
    emailVerifiedSuccess: 'toast-email-verified-success',
    emailVerifiedAlready: 'toast-email-verified-already',
    emailVerifyInvalid: 'toast-email-verify-invalid',
    emailVerifyExpired: 'toast-email-verify-expired',
    emailVerifyGenericError: 'toast-email-verify-error',
    emailVerifyRateLimit: 'toast-email-verify-rate-limit',
} as const;

type AuthToastId = keyof typeof AUTH_TOAST_IDS;

const span = (id: (typeof AUTH_TOAST_IDS)[AuthToastId], message: string) => createElement('span', { 'data-testid': id }, message);

export const authToasts = {
    loginSuccess: () => toast.success(span(AUTH_TOAST_IDS.loginSuccess, 'Logged in successfully.')),
    logoutSuccess: () => toast.success(span(AUTH_TOAST_IDS.logoutSuccess, 'Logged out successfully.')),
    sessionExpired: () => toast.error(span(AUTH_TOAST_IDS.sessionExpired, 'Session expired - please log in again.')),
    invalidCredentials: () => toast.error(span(AUTH_TOAST_IDS.loginInvalid, 'Invalid credentials or login failed.')),
    unauthorized: () => toast.error(span(AUTH_TOAST_IDS.loginUnauthorized, 'Unauthorized. Please check your credentials.')),
    tooManyAttempts: () => toast.error(span(AUTH_TOAST_IDS.loginTooManyAttempts, 'Too many login attempts. Please try again later.')),
    totpTooManyAttempts: () => toast.error(span(AUTH_TOAST_IDS.loginTotpTooManyAttempts, TOTP_TOO_MANY_ATTEMPTS_MESSAGE)),
    unexpectedError: (msg?: string) => toast.error(span(AUTH_TOAST_IDS.loginUnexpectedError, msg || 'An unexpected error occurred.')),
    totpRequired: () => toast.info(span(AUTH_TOAST_IDS.loginTOTPRequired, 'Two-factor authentication required. Please enter your code.')),
    passkeyError: (msg?: string) => toast.error(span(AUTH_TOAST_IDS.passkeyError, msg || 'Passkey authentication failed')),
    passwordResetSuccess: () => toast.success(span(AUTH_TOAST_IDS.passwordResetSuccess, 'Password reset successful. You can now log in.')),
    accountDeletedSuccess: () => toast.success(span(AUTH_TOAST_IDS.accountDeletedSuccess, 'Account deleted successfully.')),
    accountCreatedSuccess: () => toast.success(span(AUTH_TOAST_IDS.accountCreatedSuccess, 'Account created successfully. Please log in.')),
    emailVerifiedSuccess: () => toast.success(span(AUTH_TOAST_IDS.emailVerifiedSuccess, 'Email verified successfully. You can now log in.')),
    emailVerifiedAlready: () => toast.info(span(AUTH_TOAST_IDS.emailVerifiedAlready, 'Email already verified. You can log in.')),
    emailVerifyInvalid: () => toast.error(span(AUTH_TOAST_IDS.emailVerifyInvalid, 'Invalid or unknown verification token.')),
    emailVerifyExpired: () => toast.error(span(AUTH_TOAST_IDS.emailVerifyExpired, 'Verification link expired. Request a new one.')),
    emailVerifyGenericError: () => toast.error(span(AUTH_TOAST_IDS.emailVerifyGenericError, 'Email verification failed. Please request a new code or retry.')),
    emailVerifyRateLimit: () => toast.error(span(AUTH_TOAST_IDS.emailVerifyRateLimit, 'Too many verification attempts. Please wait and try again.')),
};

export type AuthToastHelpers = typeof authToasts;
