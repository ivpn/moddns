import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import QueryLogsSection from '@/pages/settings/QueryLogsSection';
import { vi, describe, it, expect } from 'vitest';

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

// Provide minimal props
const logsSettings = [
    { value: 'enable', options: [{ value: 'enable', label: 'Enable' }, { value: 'disable', label: 'Disable' }] },
    { title: 'Include blocked', description: 'Include blocked queries', value: 'on', options: [{ value: 'on', label: 'On' }] },
    { title: 'Include processed', description: 'Include processed queries', value: 'on', options: [{ value: 'on', label: 'On' }] },
    { value: '1d', options: [{ value: '1d', label: '1 Day' }] }
];

const activeProfile = { profile_id: 'profile-1' };

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
        render(<QueryLogsSection logsSettings={logsSettings} activeProfile={activeProfile} handleLogsChange={() => { }} />);

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
