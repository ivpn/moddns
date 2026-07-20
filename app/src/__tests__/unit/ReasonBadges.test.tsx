import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, test, expect } from 'vitest';
import { ReasonBadges } from '@/components/ui/ReasonBadges';

const blocklistNames = { 'hagezi-tif': 'HaGeZi TIF', x: 'Blocklist X' };
const serviceNames = { tiktok: 'TikTok' };

describe('ReasonBadges', () => {
    test('renders resolved blocklist and service names', () => {
        render(
            <ReasonBadges
                reasons={['blocklist: hagezi-tif', 'service: tiktok']}
                blocklistNames={blocklistNames}
                serviceNames={serviceNames}
            />
        );
        const badges = screen.getAllByTestId('querylog-reason-badge');
        expect(badges).toHaveLength(2);
        expect(badges[0]).toHaveTextContent('Blocklist: HaGeZi TIF');
        expect(badges[1]).toHaveTextContent('Service: TikTok');
    });

    test('falls back to the raw id when the name map has no entry', () => {
        render(<ReasonBadges reasons={['blocklist: unknown-id']} />);
        expect(screen.getByTestId('querylog-reason-badge')).toHaveTextContent('Blocklist: unknown-id');
    });

    test('renders nothing when there are no reasons', () => {
        const { container } = render(<ReasonBadges reasons={[]} />);
        expect(container).toBeEmptyDOMElement();
        expect(screen.queryByTestId('querylog-reason-badge')).not.toBeInTheDocument();
    });

    test('collapses more than three chips into a +N overflow chip', () => {
        render(
            <ReasonBadges
                reasons={[
                    'blocklist: x',
                    'service: tiktok',
                    'custom_rules',
                    'default_rule',
                ]}
                blocklistNames={blocklistNames}
                serviceNames={serviceNames}
            />
        );
        // 4 formatted chips → 3 visible + 1 overflow chip
        expect(screen.getAllByTestId('querylog-reason-badge')).toHaveLength(3);
        const overflow = screen.getByTestId('querylog-reason-badge-overflow');
        expect(overflow).toHaveTextContent('+1');
    });

    test('does not render an overflow chip when there are three or fewer chips', () => {
        render(
            <ReasonBadges
                reasons={['blocklist: x', 'service: tiktok', 'default_rule']}
                blocklistNames={blocklistNames}
                serviceNames={serviceNames}
            />
        );
        expect(screen.getAllByTestId('querylog-reason-badge')).toHaveLength(3);
        expect(screen.queryByTestId('querylog-reason-badge-overflow')).not.toBeInTheDocument();
    });
});
