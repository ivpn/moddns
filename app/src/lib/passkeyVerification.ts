import type { ResponsesPasskey } from '@/api/client/api';

// Announced date from which every passkey sign-in must verify the user. Temporary: removed with
// the notice once login requires user verification.
export const PASSKEY_VERIFICATION_REQUIRED_FROM = '2026-12-31';

/** The announced date as e.g. "December 31, 2026". */
export function passkeyVerificationDeadline(date: string = PASSKEY_VERIFICATION_REQUIRED_FROM): string {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' });
}

/** True when a passkey's latest sign-in didn't verify the user. */
export function hasUnverifiedPasskey(passkeys: ResponsesPasskey[] | null | undefined): boolean {
  return Array.isArray(passkeys) && passkeys.some(p => p.user_verified === false);
}
