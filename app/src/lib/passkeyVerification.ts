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

/** How the account can still sign in without its unverified passkeys, or null when it can't or the
 *  password can't be known yet (account not loaded). A passkey with no `user_verified` value counts
 *  as unverified. */
export function otherSignInMethod(
  authMethods: string[] | null | undefined,
  passkeys: ResponsesPasskey[] | null | undefined,
): 'password' | 'passkey' | null {
  if (Array.isArray(authMethods) && authMethods.includes('password')) return 'password';
  if (Array.isArray(passkeys) && passkeys.some(p => p.user_verified === true)) return 'passkey';
  return null;
}

/** The dismissal key for the current unverified passkeys: their ids, sorted. Null when one has
 *  no id, since the key could then match a different set. */
export function unverifiedPasskeysKey(passkeys: ResponsesPasskey[] | null | undefined): string | null {
  if (!Array.isArray(passkeys)) return null;
  const ids = passkeys.filter(p => p.user_verified === false).map(p => p.id);
  if (ids.some(id => !id)) return null;
  return ids.sort().join(',');
}
