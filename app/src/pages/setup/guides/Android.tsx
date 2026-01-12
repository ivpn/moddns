import { CodeBlock } from '@/components/setup/CodeBlock';

export const androidBadges = [
    { label: "Android 9 or newer" },
    { label: "DNS over TLS" },
];

interface AndroidDeps {
    dotEndpoint: string;
}

// Factory to create steps with injected DOT endpoint.
export function createAndroidSteps({ dotEndpoint }: AndroidDeps) {
    // EXACT wording required; preserve punctuation and spacing.
    return [
        {
            step: 1,
            instruction: (
                <span>
                    Open Settings
                </span>
            ),
        },
        {
            step: 2,
            instruction: (
                <span>
                    Navigate to Network & Internet (this may also be labeled as Wi-Fi & Internet or Mobile & Network).
                </span>
            ),
        },
        {
            step: 3,
            instruction: (
                <span>
                    Tap on Advanced: if you don't see Private DNS directly.
                </span>
            ),
        },
        {
            step: 4,
            instruction: (
                <span>
                    Tap on Private DNS.
                </span>
            ),
        },
        {
            step: 5,
            instruction: (
                <span>
                    Select Private DNS provider hostname.
                </span>
            ),
        },
        {
            step: 6,
            instruction: (
                <span>
                    Type in following endpoint, then press Save.
                    <div className="mt-2"><CodeBlock noWrap value={dotEndpoint} className="w-full" /></div>
                </span>
            ),
        },
    ];
}

export default {
    badges: androidBadges,
    // Consumers should call createAndroidSteps with dependencies
    steps: [],
};