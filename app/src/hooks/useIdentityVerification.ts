import { useCallback, useRef, useState } from 'react';
import type { IdentityVerification } from '@/components/dialogs/VerifyIdentityFields';
import { issueReauthToken, isReauthTokenRefusal, type IssuedReauthToken, type ReauthPurpose } from '@/lib/reauth';

/** A token is not reused this close to its expiry, so it cannot lapse in flight. */
const EXPIRY_MARGIN_MS = 15_000;

type KeyedToken = IssuedReauthToken & { key: string; purpose: ReauthPurpose };

/** Operations that use the token up only in the write that completes them, so a request they
 *  refuse (a wrong deletion code, an address already in use, a too-simple password) leaves it
 *  unused. The others use it up first, so after any failure their retry needs a new token. */
const TOKEN_KEPT_ON_REFUSAL: ReadonlySet<ReauthPurpose> = new Set<ReauthPurpose>(['email_change', 'account_deletion', 'password_set']);

/**
 * State for VerifyIdentityFields, and the reauth token each privileged request carries.
 *
 * `reauthToken(purpose)` returns the passkey's token, or exchanges the password (and 2FA code)
 * for one; `from` replaces the fields' verification for a dialog with its own password field.
 *
 * Call `requestFailed(err)` when a request that carried the token fails: the fields ask for the
 * passkey again and keep the typed 2FA code. A password-issued token whose request was refused
 * by an operation that leaves it unused (TOKEN_KEPT_ON_REFUSAL) is reused by the retry with the
 * same input, so the retry does not use up a second 2FA code; any other token is dropped.
 */
export function useIdentityVerification() {
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const [passkeyResetKey, setPasskeyResetKey] = useState(0);
    // The last token handed out, while the outcome of its request is unknown.
    const handedOut = useRef<KeyedToken | null>(null);
    // A token whose request failed without using it.
    const reusable = useRef<KeyedToken | null>(null);

    const onChange = useCallback((v: IdentityVerification | null) => {
        if (!v) handedOut.current = reusable.current = null;
        setVerification(v);
    }, []);

    const reauthToken = useCallback(async (purpose: ReauthPurpose, from?: IdentityVerification): Promise<string> => {
        const v = from ?? verification;
        if (!v) throw new Error('Verify your identity first.');
        if (v.reauthToken) return v.reauthToken;
        const key = JSON.stringify([purpose, v.currentPassword, v.otp]);
        const retry = reusable.current;
        reusable.current = handedOut.current = null;
        const token = retry?.key === key && retry.expiresAt - EXPIRY_MARGIN_MS > Date.now()
            ? retry
            : { ...(await issueReauthToken(v, purpose)), key, purpose };
        handedOut.current = token;
        return token.token;
    }, [verification]);

    const requestFailed = useCallback((err?: unknown) => {
        const token = handedOut.current;
        reusable.current = token && TOKEN_KEPT_ON_REFUSAL.has(token.purpose) && !isReauthTokenRefusal(err) ? token : null;
        handedOut.current = null;
        setPasskeyResetKey(k => k + 1);
    }, []);

    return { verification, onChange, passkeyResetKey, requestFailed, reauthToken };
}
