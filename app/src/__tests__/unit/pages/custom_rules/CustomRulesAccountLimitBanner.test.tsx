import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import CustomRulesAccountLimitBanner from '@/pages/custom_rules/CustomRulesAccountLimitBanner';
import { countAccountCustomRules } from '@/pages/custom_rules/utils';
import { useAppStore } from '@/store/general';
import type { ModelCustomRule, ModelProfile } from '@/api/client/api';

function profileWith(profileId: string, ruleCount: number): ModelProfile {
    return {
        profile_id: profileId,
        settings: {
            custom_rules: Array.from({ length: ruleCount }, () => ({}) as ModelCustomRule),
        },
    } as ModelProfile;
}

function setState(active: ModelProfile | null, profiles: ModelProfile[]) {
    useAppStore.setState({ activeProfile: active, profiles });
}

describe('countAccountCustomRules', () => {
    it('sums every profile and uses the live active profile instead of its stale stored copy', () => {
        const staleActive = profileWith('a', 10);
        const liveActive = profileWith('a', 25);
        expect(countAccountCustomRules([staleActive, profileWith('b', 5)], liveActive)).toBe(30);
    });

    it('counts profiles without settings as zero', () => {
        expect(countAccountCustomRules([{ profile_id: 'x' } as ModelProfile], null)).toBe(0);
    });
});

describe('CustomRulesAccountLimitBanner', () => {
    beforeEach(() => setState(null, []));

    it('renders nothing below 90% of the account cap', () => {
        const active = profileWith('a', 4000);
        setState(active, [active, profileWith('b', 4999)]);
        const { container } = render(<CustomRulesAccountLimitBanner />);
        expect(container).toBeEmptyDOMElement();
    });

    it('shows the account-wide usage from 9,000 rules', () => {
        const active = profileWith('a', 5000);
        setState(active, [active, profileWith('b', 4214)]);
        render(<CustomRulesAccountLimitBanner />);
        expect(
            screen.getByText(`${(9214).toLocaleString()} of ${(10000).toLocaleString()} custom rules used across all profiles.`),
        ).toBeInTheDocument();
        expect(screen.queryByText(/won't be accepted/)).not.toBeInTheDocument();
    });

    it('counts the active profile from its live rules, not the stored list', () => {
        const live = profileWith('a', 9000);
        setState(live, [profileWith('a', 100)]);
        render(<CustomRulesAccountLimitBanner />);
        expect(screen.getByText(/custom rules used across all profiles/)).toBeInTheDocument();
    });

    it('warns at the cap that new rules are refused until some are removed', () => {
        const active = profileWith('a', 6000);
        setState(active, [active, profileWith('b', 4000)]);
        render(<CustomRulesAccountLimitBanner />);
        expect(screen.getByRole('alert')).toHaveTextContent(
            `${(10000).toLocaleString()} of ${(10000).toLocaleString()} custom rules used across all profiles.`,
        );
        expect(screen.getByRole('alert')).toHaveTextContent(/won't be accepted until you remove some rules from any profile/);
    });
});
