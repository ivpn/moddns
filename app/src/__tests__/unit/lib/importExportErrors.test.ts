import { describe, it, expect } from 'vitest';
import { importErrorMessage, isReauthError } from '@/lib/importExportErrors';

describe('importErrorMessage', () => {
    it('returns file-too-large message for 413', () => {
        expect(importErrorMessage(413)).toBe('That file is too large to import.');
    });

    it('returns unsupported-type message for 415', () => {
        expect(importErrorMessage(415)).toBe('Unsupported file type.');
    });

    it('returns rate-limit message for 429', () => {
        expect(importErrorMessage(429)).toBe('Too many attempts. Please wait a little and try again.');
    });

    it('returns timeout message for 504', () => {
        expect(importErrorMessage(504)).toBe(
            'The import took too long. Try importing fewer profiles or rules at a time.'
        );
    });

    it('returns timeout message when status is undefined (network error)', () => {
        expect(importErrorMessage(undefined)).toBe(
            'The import took too long. Try importing fewer profiles or rules at a time.'
        );
    });

    it('prefers serverError for 400 when present', () => {
        expect(importErrorMessage(400, 'file schema is invalid')).toBe('file schema is invalid');
    });

    it('falls back to generic 400 message when serverError is absent', () => {
        expect(importErrorMessage(400)).toBe(
            'The file could not be imported. Please check it and try again.'
        );
    });

    it('prefers serverError for an unrecognised status', () => {
        expect(importErrorMessage(500, 'internal error')).toBe('internal error');
    });

    it('falls back to generic message for unrecognised status without serverError', () => {
        expect(importErrorMessage(500)).toBe(
            'The file could not be imported. Please check it and try again.'
        );
    });
});

describe('isReauthError', () => {
    it('returns true when message contains "password"', () => {
        expect(isReauthError('invalid current password')).toBe(true);
    });

    it('returns true when message contains "reauth"', () => {
        expect(isReauthError('reauth required')).toBe(true);
    });

    it('returns true when message contains "token"', () => {
        expect(isReauthError('invalid token')).toBe(true);
    });

    it('is case-insensitive', () => {
        expect(isReauthError('Invalid Current Password')).toBe(true);
    });

    it('returns false for a generic validation message', () => {
        expect(isReauthError('file schema is invalid')).toBe(false);
    });

    it('returns false for undefined', () => {
        expect(isReauthError(undefined)).toBe(false);
    });

    it('returns false for empty string', () => {
        expect(isReauthError('')).toBe(false);
    });
});
