import { useEffect, useRef } from 'react';
import api from '@/api/api';
import type { ResponsesPasskey } from '@/api/client/api';
import { AUTH_KEY } from '@/lib/consts';
import { useAppStore } from '@/store/general';

/** Loads the passkey list once per signed-in layout mount. Some routes don't load the
 *  account; then the list is loaded without knowing whether the account has a passkey. */
export function useLoadPasskeysOnce(signedIn: boolean) {
    const account = useAppStore(s => s.account);
    const setPasskeys = useAppStore(s => s.setPasskeys);
    const loaded = useRef(false);
    const noPasskey = !!account && !(Array.isArray(account.auth_methods) && account.auth_methods.includes('passkey'));

    useEffect(() => {
        if (loaded.current || !signedIn || noPasskey || localStorage.getItem(AUTH_KEY) !== 'true') return;
        loaded.current = true;
        const { authEpoch: epoch, passkeys: before } = useAppStore.getState();
        api.Client.authApi.apiV1WebauthnPasskeysGet()
            .then(res => {
                const state = useAppStore.getState();
                // Drop a list that arrives after logout, or after anything stored a newer list
                // (every update replaces the array, an empty one included).
                if (state.authEpoch !== epoch || state.passkeys !== before || !Array.isArray(res.data)) return;
                setPasskeys(res.data as ResponsesPasskey[]);
            })
            .catch(() => { }); // the banner is advisory; Passkey settings reports load errors
    }, [signedIn, noPasskey, setPasskeys]);
}
