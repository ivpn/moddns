/**
 * Unit tests for useProfileExport blob-error parsing (F1).
 *
 * The export API request uses responseType:'blob', so axios delivers error
 * bodies as Blobs. We verify the hook parses them correctly and:
 *   - parses the Blob body and surfaces the server message via a toast
 *     (including a refused reauth token and 401 failures)
 *   - falls back to a generic message when the Blob is not JSON
 *
 * jsdom does not implement Blob.text() — polyfill it before importing the hook.
 */
if (!Blob.prototype.text) {
    Blob.prototype.text = function () {
        return new Promise<string>((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.onerror = () => reject(reader.error);
            reader.readAsText(this);
        });
    };
}

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useProfileExport } from '@/hooks/useProfileExport';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: {
                apiV1ProfilesExportPost: vi.fn(),
            },
        },
    },
}));

vi.mock('sonner', () => ({
    toast: {
        error: vi.fn(),
        success: vi.fn(),
    },
}));

// jsdom doesn't implement URL.createObjectURL — stub it
Object.assign(globalThis.URL, {
    createObjectURL: vi.fn(() => 'blob:fake-url'),
    revokeObjectURL: vi.fn(),
});

import API from '@/api/api';
import { toast } from 'sonner';

const mockPost = vi.mocked(API.Client.profilesApi.apiV1ProfilesExportPost);
const mockToastError = vi.mocked(toast.error);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeAxiosError(status: number, body: object | string) {
    const blob =
        typeof body === 'string'
            ? new Blob([body], { type: 'text/plain' })
            : new Blob([JSON.stringify(body)], { type: 'application/json' });

    const err = Object.assign(new Error('Request failed'), {
        response: { status, data: blob },
    });
    return err;
}

const baseInput = {
    scope: 'all' as const,
    reauthToken: () => Promise.resolve('reauth-token'),
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useProfileExport — blob error parsing', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('parses JSON blob on a refused reauth token and toasts the server message', async () => {
        mockPost.mockRejectedValueOnce(
            makeAxiosError(400, { error: 'invalid reauth token' })
        );

        const { result } = renderHook(() => useProfileExport());

        let caught: Error | null = null;
        await act(async () => {
            try {
                await result.current.exportProfiles(baseInput);
            } catch (e) {
                caught = e as Error;
            }
        });

        expect(caught).not.toBeNull();
        expect(mockToastError).toHaveBeenCalledWith('invalid reauth token');
    });

    it('toasts the parsed server message on a 401 reauth failure', async () => {
        mockPost.mockRejectedValueOnce(
            makeAxiosError(401, { error: 'authentication required' })
        );

        const { result } = renderHook(() => useProfileExport());

        await act(async () => {
            try {
                await result.current.exportProfiles(baseInput);
            } catch {
                // expected
            }
        });

        expect(mockToastError).toHaveBeenCalledWith('authentication required');
    });

    it('fires toast with server message for a non-reauth error (e.g. 500)', async () => {
        mockPost.mockRejectedValueOnce(
            makeAxiosError(500, { error: 'internal server error' })
        );

        const { result } = renderHook(() => useProfileExport());

        await act(async () => {
            try {
                await result.current.exportProfiles(baseInput);
            } catch {
                // expected
            }
        });

        expect(mockToastError).toHaveBeenCalledWith('internal server error');
    });

    it('falls back to generic message when blob is not JSON', async () => {
        // Non-JSON blob
        const err = Object.assign(new Error('Request failed'), {
            response: {
                status: 503,
                data: new Blob(['<html>Bad Gateway</html>'], { type: 'text/html' }),
            },
        });
        mockPost.mockRejectedValueOnce(err);

        const { result } = renderHook(() => useProfileExport());

        await act(async () => {
            try {
                await result.current.exportProfiles(baseInput);
            } catch {
                // expected
            }
        });

        expect(mockToastError).toHaveBeenCalledWith('Export failed. Please try again.');
    });
});

// The request carries only the reauth token; the password and the 2FA code are
// exchanged for it first. specRef: api-endpoint-behaviour E6
describe('useProfileExport - reauth token', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mockPost.mockResolvedValue({ data: new Blob(['{}']), headers: {} } as never);
        // The hook downloads through a link click, which jsdom cannot navigate.
        vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    });

    it('sends the reauth token in the body', async () => {
        const { result } = renderHook(() => useProfileExport());
        await act(async () => {
            await result.current.exportProfiles(baseInput);
        });
        const [body, options] = mockPost.mock.calls[0];
        expect(body).toEqual({ scope: 'all', reauth_token: 'reauth-token' });
        expect(options).toEqual({ responseType: 'blob' });
    });

    it('toasts a refused password and sends no export request', async () => {
        const refused = Object.assign(new Error('Request failed'), { response: { status: 400, data: { error: 'invalid current password' } } });
        const { result } = renderHook(() => useProfileExport());
        await act(async () => {
            await expect(result.current.exportProfiles({ scope: 'all', reauthToken: () => Promise.reject(refused) })).rejects.toBe(refused);
        });
        expect(mockPost).not.toHaveBeenCalled();
        expect(mockToastError).toHaveBeenCalledWith('invalid current password');
    });
});
