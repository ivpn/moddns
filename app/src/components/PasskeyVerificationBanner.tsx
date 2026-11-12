import { Link } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { useAppStore } from '@/store/general';
import { hasUnverifiedPasskey, passkeyVerificationDeadline } from '@/lib/passkeyVerification';

/** Shown at the top of every signed-in page while one of the account's passkeys doesn't verify the user. */
export default function PasskeyVerificationBanner() {
    const passkeys = useAppStore(s => s.passkeys);
    if (!hasUnverifiedPasskey(passkeys)) return null;

    return (
        <div
            data-testid="passkey-verification-banner"
            className="flex items-start gap-3 w-full rounded-lg border-2 border-yellow-600 dark:border-yellow-400 bg-yellow-500/10 px-4 py-3"
        >
            <AlertTriangle className="w-5 h-5 mt-0.5 flex-shrink-0 text-yellow-600 dark:text-yellow-400" aria-hidden="true" />
            <div className="min-w-0 font-['Figtree',Helvetica] text-sm leading-5">
                <p className="font-semibold text-[var(--shadcn-ui-app-foreground)]">
                    One of your passkeys needs an important update.
                </p>
                <p className="mt-0.5 text-[var(--shadcn-ui-app-foreground)]">
                    From {passkeyVerificationDeadline()}, signing in with a passkey will ask for your fingerprint, face, device password or security key PIN, and a passkey that can't will stop working.{' '}
                    <Link to="/faq#passkey-pin" className="font-semibold !underline !text-[var(--tailwind-colors-rdns-600)] hover:!text-[var(--tailwind-colors-rdns-700)]">
                        See what to do
                    </Link>
                </p>
            </div>
        </div>
    );
}
