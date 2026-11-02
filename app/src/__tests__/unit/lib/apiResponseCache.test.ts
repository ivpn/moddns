import { describe, it, expect, vi, afterEach } from 'vitest';
import { deleteApiResponseCache } from '@/lib/apiResponseCache';

describe('deleteApiResponseCache', () => {
    afterEach(() => {
        vi.unstubAllGlobals();
    });

    it('deletes the api runtime cache', async () => {
        const deleteMock = vi.fn().mockResolvedValue(true);
        vi.stubGlobal('caches', { delete: deleteMock });
        await deleteApiResponseCache();
        expect(deleteMock).toHaveBeenCalledWith('api');
    });

    it('resolves when the cache does not exist', async () => {
        vi.stubGlobal('caches', { delete: vi.fn().mockResolvedValue(false) });
        await expect(deleteApiResponseCache()).resolves.toBeUndefined();
    });

    it('is a no-op when Cache Storage is unavailable', async () => {
        vi.stubGlobal('caches', undefined);
        await expect(deleteApiResponseCache()).resolves.toBeUndefined();
    });

    it('swallows Cache Storage errors', async () => {
        vi.stubGlobal('caches', { delete: vi.fn().mockRejectedValue(new DOMException('denied', 'SecurityError')) });
        await expect(deleteApiResponseCache()).resolves.toBeUndefined();
    });
});
