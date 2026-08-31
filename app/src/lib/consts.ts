export const AUTH_KEY = "isAuthenticated";
export const PASSWORD_COMPLEXITY_RULES = "Password must be 12-64 characters, contain at least one uppercase letter, one lowercase letter, one number, and one special character."

// Query-logs auto-refresh intervals (Grafana-style split refresh button).
// "auto" is the default cadence; ms: null = auto-refresh off.
export type RefreshIntervalKey = "off" | "auto" | "5s" | "10s" | "15s" | "30s" | "60s";
export interface RefreshIntervalOption {
    key: RefreshIntervalKey;
    /** Menu entry label. */
    label: string;
    /** Compact label shown on the split button while active. */
    buttonLabel: string;
    ms: number | null;
}
export const QUERY_LOGS_REFRESH_INTERVALS: RefreshIntervalOption[] = [
    { key: "off", label: "Off", buttonLabel: "", ms: null },
    { key: "auto", label: "Auto (10s)", buttonLabel: "Auto", ms: 10_000 },
    { key: "5s", label: "5s", buttonLabel: "5s", ms: 5_000 },
    { key: "10s", label: "10s", buttonLabel: "10s", ms: 10_000 },
    { key: "15s", label: "15s", buttonLabel: "15s", ms: 15_000 },
    { key: "30s", label: "30s", buttonLabel: "30s", ms: 30_000 },
    { key: "60s", label: "60s", buttonLabel: "60s", ms: 60_000 },
];
export const refreshIntervalMsFor = (key: RefreshIntervalKey): number | null =>
    QUERY_LOGS_REFRESH_INTERVALS.find(option => option.key === key)?.ms ?? null;