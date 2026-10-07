import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import QueryLogsSection from '@/pages/settings/QueryLogsSection';
import { vi, describe, it, expect } from 'vitest';
import type { ModelProfile } from '@/api/client';

// Mock api client
vi.mock('@/api/api', () => ({
    default: {
        Client: {
            queryLogsApi: {
                apiV1ProfilesIdLogsDownloadGet: vi.fn().mockResolvedValue({
                    data: [
                        { timestamp: '2025-11-13T00:00:00Z', status: 'processed', protocol: 'udp' }
                    ],
                    headers: { 'content-disposition': 'attachment; filename="dns-query-logs.json"' }
                }),
                apiV1ProfilesIdLogsDelete: vi.fn().mockResolvedValue({})
            }
        }
    }
}));

const activeProfile = {
    id: 'profile-1',
    profile_id: 'profile-1',
    account_id: 'a',
    name: 'p',
    settings: {},
} as unknown as ModelProfile;

// Mock URL + anchor interactions (JSDOM lacks createObjectURL)
const createObjectURLMock = vi.fn().mockReturnValue('blob:url');
Object.defineProperty(URL, 'createObjectURL', { value: createObjectURLMock });

// Intercept anchor click creation
let capturedAnchor: HTMLAnchorElement | null = null;
const originalAppendChild = document.body.appendChild.bind(document.body);
document.body.appendChild = (<T extends Node>(el: T): T => {
    if ((el as unknown as HTMLElement).tagName === 'A') {
        capturedAnchor = el as unknown as HTMLAnchorElement;
        vi.spyOn(el as unknown as HTMLAnchorElement, 'click').mockImplementation(() => { });
    }
    return originalAppendChild(el);
}) as typeof document.body.appendChild;

describe('DownloadQueryLogsButton', () => {
    it('calls API and creates a downloadable blob with expected filename', async () => {
        render(<QueryLogsSection activeProfile={activeProfile} />);

        const btn = screen.getByText('Download query logs');
        fireEvent.click(btn);

        await waitFor(() => {
            // Ensure object URL was created and anchor prepared
            expect(createObjectURLMock).toHaveBeenCalledTimes(1);
            expect(capturedAnchor).not.toBeNull();
            expect(capturedAnchor!.getAttribute('download')).toBe('dns-query-logs.json');
        });
    });
});
