import { useCallback, useEffect, useRef, useState } from "react";

export interface ResourceState<T> {
    status: "idle" | "loading" | "ready" | "error";
    data: T | null;
    /** A newer request is in flight while `data` is still the previous answer. */
    refreshing: boolean;
    /** HTTP status of a failed request, when there was one. */
    errorStatus: number | null;
}

const STALE_AFTER_MS = 5 * 60 * 1000;

/**
 * Fetches `fetcher()` whenever `key` changes.
 *
 * `scope` identifies what the data belongs to (the profile); a new scope drops the previous data
 * (skeletons), while a new key within the same scope keeps it and marks it `refreshing` (K8, P14).
 * Answers for a key that is no longer current are ignored. A silent refetch runs when the tab
 * becomes visible after more than five minutes (K9).
 */
export function useApiResource<T>(
    scope: string | null,
    key: string | null,
    fetcher: () => Promise<T>,
): ResourceState<T> & { reload: () => void } {
    const [state, setState] = useState<ResourceState<T>>({ status: "idle", data: null, refreshing: false, errorStatus: null });
    const keyRef = useRef(key);
    keyRef.current = key;
    const scopeRef = useRef<string | null>(null);
    const fetcherRef = useRef(fetcher);
    fetcherRef.current = fetcher;
    const lastOk = useRef(0);

    const run = useCallback(async (k: string, silent: boolean) => {
        try {
            const data = await fetcherRef.current();
            if (keyRef.current !== k) return;
            lastOk.current = Date.now();
            setState({ status: "ready", data, refreshing: false, errorStatus: null });
        } catch (e) {
            if (keyRef.current !== k || silent) return;
            const status = (e as { response?: { status?: number } })?.response?.status ?? null;
            setState({ status: "error", data: null, refreshing: false, errorStatus: status });
        }
    }, []);

    useEffect(() => {
        if (!key) {
            scopeRef.current = null;
            setState({ status: "idle", data: null, refreshing: false, errorStatus: null });
            return;
        }
        const sameScope = scopeRef.current === scope;
        scopeRef.current = scope;
        setState(s =>
            sameScope && s.data !== null
                ? { ...s, refreshing: true }
                : { status: "loading", data: null, refreshing: false, errorStatus: null },
        );
        void run(key, false);
    }, [key, scope, run]);

    useEffect(() => {
        const onVisible = () => {
            if (document.visibilityState !== "visible" || !keyRef.current) return;
            if (Date.now() - lastOk.current > STALE_AFTER_MS) void run(keyRef.current, true);
        };
        document.addEventListener("visibilitychange", onVisible);
        return () => document.removeEventListener("visibilitychange", onVisible);
    }, [run]);

    const reload = useCallback(() => {
        if (!keyRef.current) return;
        setState({ status: "loading", data: null, refreshing: false, errorStatus: null });
        void run(keyRef.current, false);
    }, [run]);

    return { ...state, reload };
}
