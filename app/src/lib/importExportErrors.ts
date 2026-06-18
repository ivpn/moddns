/**
 * Centralised error-message mapping for the import and export flows.
 *
 * The import hook receives a plain JSON error body so `serverError` is already
 * available. The export hook uses responseType:'blob', meaning the error body
 * arrives as a Blob and must be read before calling here — see useProfileExport.
 */

/** Human-readable message for an import (or export) failure. */
export function importErrorMessage(status?: number, serverError?: string): string {
    switch (status) {
        case 413:
            return 'That file is too large to import.';
        case 415:
            return 'Unsupported file type.';
        case 429:
            return 'Too many attempts. Please wait a little and try again.';
        case 504:
            return 'The import took too long. Try importing fewer profiles or rules at a time.';
        default:
            if (!status) {
                // Network error / no response (timeout, offline, etc.)
                return 'The import took too long. Try importing fewer profiles or rules at a time.';
            }
            // For 400 and any other status: prefer the server message when present.
            return serverError ?? 'The file could not be imported. Please check it and try again.';
    }
}

/**
 * Return true when a server error string looks like a reauth / password
 * failure rather than a generic data-validation 400.
 */
export function isReauthError(serverError?: string): boolean {
    if (!serverError) return false;
    const lower = serverError.toLowerCase();
    return lower.includes('password') || lower.includes('reauth') || lower.includes('token');
}
