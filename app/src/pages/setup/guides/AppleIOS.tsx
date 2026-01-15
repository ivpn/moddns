import React from 'react';

export default function IOS(): React.ReactNode {
    return (
        <div className="mt-3 pt-3 border-t border-[var(--tailwind-colors-rdns-700)]">
            <ol className="mt-2 list-decimal pl-5 space-y-2 text-sm text-[var(--tailwind-colors-slate-200)]">
                <li>Download the configuration profile to your device.</li>
                <li>Open Settings.</li>
                <li>
                    Locate the Profile: Look for a banner near the top that says "Profile Downloaded"
                    <ul className="mt-1 list-disc pl-5 text-xs text-[var(--tailwind-colors-slate-300)]">
                        <li>If you don't see it, go to Settings &gt; General &gt; VPN &amp; Device Management</li>
                    </ul>
                </li>
                <li>Tap Profile Downloaded.</li>
                <li>Tap Install in the top-right corner and complete the prompts.</li>
            </ol>
        </div>
    );
}
