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

const mockPost = vi.mocked(API.Client.profilesApi.apiV1ProfilesImportPost);
const payload = { schemaVersion: 1, kind: 'moddns-export', profiles: [] } as unknown as ModelExportEnvelope;

// The 2FA code goes in the request body with the password, never in a header,
// and is left out with a passkey reauth token. specRef: api-endpoint-behaviour E6
describe('useProfileImport - 2FA code', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mockPost.mockResolvedValue({ data: { createdProfileIds: [], createdProfileNames: [], warnings: [] } } as never);
    });

    it('sends the code in the body with the password', async () => {
        const { result } = renderHook(() => useProfileImport());
        await act(async () => {
            await result.current.importProfiles({ payload, currentPassword: 'secret', otp: '123456' });
        });
        expect(mockPost.mock.calls[0]).toEqual(['confirm', { mode: 'create_new', payload, current_password: 'secret', otp: '123456' }]);
    });

    it('leaves the code out with a passkey reauth token', async () => {
        const { result } = renderHook(() => useProfileImport());
        await act(async () => {
            await result.current.importProfiles({ payload, reauthToken: 'reauth-token', otp: '123456' });
        });
        expect(mockPost.mock.calls[0]).toEqual(['confirm', { mode: 'create_new', payload, reauth_token: 'reauth-token' }]);
    });
});
