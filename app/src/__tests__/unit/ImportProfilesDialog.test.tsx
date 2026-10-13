// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ImportProfilesDialog from '@/components/dialogs/ImportProfilesDialog';
import { useAppStore } from '@/store/general';

vi.mock('sonner', () => ({
    toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            profilesApi: { apiV1ProfilesGet: vi.fn() },
            accountsApi: {},
        },
    },
}));

vi.mock('@/lib/webauthn', () => ({
    beginProfileImportReauth: vi.fn(),
}));

const envelope = {
    schemaVersion: 1,
    kind: 'moddns-export',
    profiles: [
        { name: 'Home', settings: { logs: { enabled: true, logDomains: true, retention: '1d' } } },
        { name: 'Quiet', settings: { statistics: { enabled: false }, logs: { enabled: false } } },
        { name: 'Kids', settings: { statistics: { enabled: true } } },
    ],
};

const NOTICE = 'import-data-collection-warning';

async function openConfirmStep(payload: unknown = envelope) {
    const user = userEvent.setup();
    render(<ImportProfilesDialog open onOpenChange={vi.fn()} />);
    const input = screen.getByTestId('import-dropzone').querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File([JSON.stringify(payload)], 'backup.moddns.json', { type: 'application/json' });
    await user.upload(input, file);
    await screen.findByText('Profile selection');
    return user;
}

const lines = () => within(screen.getByTestId(NOTICE)).getAllByRole('listitem').map(li => li.textContent);

describe('ImportProfilesDialog data collection notice', () => {
    beforeEach(() => {
        useAppStore.setState({ account: null, passkeys: [], profiles: [] });
    });

    it('lists each selected collecting profile in file order with what it starts collecting', async () => {
        // tableRef: account-export-import-behaviour #I26
        await openConfirmStep();
        const notice = screen.getByTestId(NOTICE);
        expect(within(notice).getByText('Data collection starts on import')).toBeInTheDocument();
        expect(
            within(notice).getByText("These profiles start collecting as soon as they are imported. You can turn it off in each profile's Settings."),
        ).toBeInTheDocument();
        expect(lines()).toEqual([
            'Home: a record of each query with its domain, kept for 1 day.',
            'Kids: query counts per device, kept for 30 days.',
        ]);
    });

    it('updates with the selection and disappears when no selected profile collects anything', async () => {
        // tableRef: account-export-import-behaviour #I26
        const user = await openConfirmStep();
        await user.click(screen.getByRole('checkbox', { name: 'Select profile Home' }));
        expect(lines()).toEqual(['Kids: query counts per device, kept for 30 days.']);

        await user.click(screen.getByRole('checkbox', { name: 'Select profile Kids' }));
        expect(screen.queryByTestId(NOTICE)).not.toBeInTheDocument();
        expect(screen.queryByText('Data collection starts on import')).not.toBeInTheDocument();

        await user.click(screen.getByRole('checkbox', { name: 'Select profile Home' }));
        expect(lines()).toEqual(['Home: a record of each query with its domain, kept for 1 day.']);
    });

    it('is absent when the file has no collecting profile', async () => {
        // tableRef: account-export-import-behaviour #I26
        await openConfirmStep({ ...envelope, profiles: [envelope.profiles[1]] });
        expect(screen.queryByTestId(NOTICE)).not.toBeInTheDocument();
    });
});
