// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import MainContentSection from '@/pages/custom_rules/MainContentSection';
import { customRulesPath, parseRuleList } from '@/pages/custom_rules/utils';
import { useAppStore } from '@/store/general';
import type { ModelProfile } from '@/api/client';

vi.mock('@/api/api', () => ({ default: { Client: { profilesApi: {} } } }));

const profile = {
    id: 'p1',
    profile_id: 'p1',
    account_id: 'a',
    name: 'Home',
    settings: {
        custom_rules: [
            { id: 'r1', action: 'block', value: 'bad.example', order: 0 },
            { id: 'r2', action: 'allow', value: 'good.example', order: 0 },
        ],
    },
} as unknown as ModelProfile;

function mount(url: string) {
    useAppStore.setState({ activeProfile: profile, profiles: [profile], subscriptionStatus: null });
    return render(
        <MemoryRouter initialEntries={[url]}>
            <MainContentSection profiles={[profile]} />
        </MemoryRouter>,
    );
}

beforeEach(() => {
    useAppStore.setState({ activeProfile: null });
});

describe('custom rules ?list= param', () => {
    it('parses the list and builds the path', () => {
        // tableRef: statistics-behaviour #P26
        expect(parseRuleList('allowlist')).toBe('allowlist');
        expect(parseRuleList('denylist')).toBe('denylist');
        expect(parseRuleList('bogus')).toBe('denylist');
        expect(parseRuleList(null)).toBe('denylist');
        expect(customRulesPath('allowlist')).toBe('/custom-rules?list=allowlist');
        expect(customRulesPath('denylist')).toBe('/custom-rules?list=denylist');
    });

    it.each([
        ['/custom-rules?list=allowlist', 'Allowlist'],
        ['/custom-rules?list=denylist', 'Denylist'],
        ['/custom-rules?list=bogus', 'Denylist'],
        ['/custom-rules', 'Denylist'],
    ])('opens %s on the %s tab', (url, tab) => {
        // tableRef: statistics-behaviour #P26
        mount(url);
        expect(screen.getByRole('tab', { name: tab })).toHaveAttribute('data-state', 'active');
    });
});
