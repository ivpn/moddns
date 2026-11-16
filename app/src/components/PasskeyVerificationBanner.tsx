import { Link } from 'react-router-dom';
import { AlertTriangle, X } from 'lucide-react';
import { useAppStore } from '@/store/general';
import {
    hasUnverifiedPasskey,
    otherSignInMethod,
    passkeyVerificationDeadline,
    unverifiedPasskeysKey,
} from '@/lib/passkeyVerification';

/** Shown at the top of every signed-in page while one of the account's passkeys doesn't verify the user.
 *  It can be closed only when the account can still sign in another way. */
export default function PasskeyVerificationBanner() {
    const passkeys = useAppStore(s => s.passkeys);
    const authMethods = useAppStore(s => s.account?.auth_methods);
    const dismissedKey = useAppStore(s => s.passkeyBannerDismissedKey);
    const setDismissedKey = useAppStore(s => s.setPasskeyBannerDismissedKey);
    if (!hasUnverifiedPasskey(passkeys)) return null;

    const key = unverifiedPasskeysKey(passkeys);
    const fallback = key === null ? null : otherSignInMethod(authMethods, passkeys);
    // Some routes don't load the account; a dismissal stands there until the account shows otherwise.
    const accountLoaded = Array.isArray(authMethods);
    if (key !== null && (fallback || !accountLoaded) && dismissedKey === key) return null;

    const close = () => {
        if (key !== null) setDismissedKey(key);
    };

    return (
        <div
            data-testid="passkey-verification-banner"
            className="flex items-start gap-3 w-full rounded-lg border-2 border-yellow-600 dark:border-yellow-400 bg-yellow-500/10 px-4 py-3"
        >
            <AlertTriangle className="w-5 h-5 mt-0.5 flex-shrink-0 text-yellow-600 dark:text-yellow-400" aria-hidden="true" />
            <div className="min-w-0 flex-1 font-['Figtree',Helvetica] text-sm leading-5">
                <p className="font-semibold text-[var(--shadcn-ui-app-foreground)]">
                    One of your passkeys needs an important update.
                </p>
                <p className="mt-0.5 text-[var(--shadcn-ui-app-foreground)]">
                    From {passkeyVerificationDeadline()}, signing in with a passkey will ask for your fingerprint, face, device password or security key PIN, and a passkey that can't will stop working.{' '}
                    {fallback === 'password' && 'You can still sign in with your password. '}
                    {fallback === 'passkey' && 'You can still sign in with your other passkey. '}
                    <Link to="/faq#passkey-pin" className="font-semibold !underline !text-[var(--tailwind-colors-rdns-600)] hover:!text-[var(--tailwind-colors-rdns-700)]">
                        See what to do
                    </Link>
                </p>
            </div>
            {fallback && (
                <button
                    type="button"
                    aria-label="Close"
                    onClick={close}
                    data-testid="passkey-verification-banner-close"
                    className="flex-shrink-0 -mr-2 -my-1.5 flex h-8 w-8 items-center justify-center rounded-md text-[var(--shadcn-ui-app-foreground)] hover:bg-yellow-500/20 cursor-pointer"
                >
                    <X className="w-4 h-4" aria-hidden="true" />
                </button>
            )}
        </div>
    );
}
