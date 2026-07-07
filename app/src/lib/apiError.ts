/**
 * Shared helper for turning an Axios/API error into a user-facing message.
 *
 * The backend returns errors as `{ error: string }` (sometimes `message` /
 * `detail`), and several of those strings are already phrased as user-facing
 * copy (e.g. the custom-rule limit message). This helper surfaces that text,
 * falling back to a caller-supplied default.
 */

export interface ApiErrorLike {
    response?: {
        data?: {
            error?: string;
            message?: string;
            detail?: string;
        };
    };
    message?: string;
}

export const formatApiError = (error: unknown, fallback: string): string => {
    if (typeof error === "string") {
        return error;
    }

    if (error && typeof error === "object") {
        const err = error as ApiErrorLike;
        const data = err.response?.data;
        return data?.error ?? data?.message ?? data?.detail ?? err.message ?? fallback;
    }

    return fallback;
};
