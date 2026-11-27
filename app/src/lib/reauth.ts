import axios from 'axios';
import api from '@/api/api';
import type { RequestsReauthRequestPurposeEnum } from '@/api/client/api';
import type { IdentityVerification } from '@/components/dialogs/VerifyIdentityFields';

export type ReauthPurpose = RequestsReauthRequestPurposeEnum;

export interface IssuedReauthToken {
  token: string;
  /** Epoch milliseconds. */
  expiresAt: number;
}

/** Operations that ask for the 2FA code themselves: their token is issued for the password alone. */
const CODE_CHECKED_BY_OPERATION: ReadonlySet<ReauthPurpose> = new Set<ReauthPurpose>(['totp_disable', 'backup_codes_regenerate']);

/** Exchanges the current password (plus the 2FA code when 2FA is on) for a single-use reauth token. */
export async function issueReauthToken(verification: IdentityVerification, purpose: ReauthPurpose): Promise<IssuedReauthToken> {
  const otp = CODE_CHECKED_BY_OPERATION.has(purpose) ? undefined : verification.otp;
  const response = await api.Client.accountsApi.apiV1AccountsReauthPost({
    purpose,
    current_password: verification.currentPassword ?? '',
    ...(otp && { otp }),
  });
  const { reauth_token: token, expires_at: expiresAt } = response.data;
  if (!token) throw new Error('Missing reauth token');
  return { token, expiresAt: expiresAt ? Date.parse(expiresAt) : Date.now() };
}

const TOKEN_REFUSALS = ['missing reauth_token', 'invalid reauth token', 'reauth token expired'];

/** Whether a failed request refused its reauth token (used up, expired or missing). The error is
 *  the API's response, or an Error carrying its message. */
export function isReauthTokenRefusal(err: unknown): boolean {
  if (axios.isAxiosError(err)) {
    const message = (err.response?.data as { error?: unknown } | undefined)?.error;
    return err.response?.status === 400 && typeof message === 'string' && TOKEN_REFUSALS.includes(message);
  }
  return err instanceof Error && TOKEN_REFUSALS.includes(err.message);
}
