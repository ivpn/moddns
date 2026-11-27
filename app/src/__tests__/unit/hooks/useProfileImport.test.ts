import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useProfileImport } from '@/hooks/useProfileImport';
import type { ModelExportEnvelope } from '@/api/client/api';

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: {
                apiV1ProfilesImportPost: vi.fn(),
            },
        },
    },
}));

vi.mock('sonner', () => ({
    toast: { error: vi.fn(), success: vi.fn() },
}));

import API from '@/api/api';
import { toast } from 'sonner';

const mockPost = vi.mocked(API.Client.profilesApi.apiV1ProfilesImportPost);
const payload = { schemaVersion: 1, kind: 'moddns-export', profiles: [] } as unknown as ModelExportEnvelope;

// The request carries only the reauth token; the password and the 2FA code are
// exchanged for it first. specRef: api-endpoint-behaviour E6
describe('useProfileImport - reauth token', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mockPost.mockResolvedValue({ data: { createdProfileIds: [], createdProfileNames: [], warnings: [] } } as never);
    });

    it('sends the reauth token in the body', async () => {
        const { result } = renderHook(() => useProfileImport());
        await act(async () => {
            await result.current.importProfiles({ payload, reauthToken: () => Promise.resolve('reauth-token') });
        });
        expect(mockPost.mock.calls[0]).toEqual(['confirm', { mode: 'create_new', payload, reauth_token: 'reauth-token' }]);
    });

    it('toasts a refused password and sends no import request', async () => {
        const refused = Object.assign(new Error('Request failed'), { response: { status: 400, data: { error: 'invalid current password' } } });
        const { result } = renderHook(() => useProfileImport());
        await act(async () => {
            await expect(result.current.importProfiles({ payload, reauthToken: () => Promise.reject(refused) })).rejects.toBe(refused);
        });
        expect(mockPost).not.toHaveBeenCalled();
        expect(vi.mocked(toast.error)).toHaveBeenCalledWith('invalid current password');
    });
});
