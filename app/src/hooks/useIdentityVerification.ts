import { useCallback, useState } from 'react';
import type { IdentityVerification } from '@/components/dialogs/VerifyIdentityFields';

/**
 * State for VerifyIdentityFields. Call `requestFailed` when a request that carried the
 * verification fails: the server uses a passkey reauth token up on the first try, so the
 * fields ask for the passkey again and keep the typed 2FA code.
 */
export function useIdentityVerification() {
    const [verification, setVerification] = useState<IdentityVerification | null>(null);
    const [passkeyResetKey, setPasskeyResetKey] = useState(0);
    const onChange = useCallback((v: IdentityVerification | null) => setVerification(v), []);
    const requestFailed = useCallback(() => setPasskeyResetKey(k => k + 1), []);
    return { verification, onChange, passkeyResetKey, requestFailed };
}
