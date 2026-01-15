import React from 'react';

export default function MacOS(): React.ReactNode {
    return (
        <div className="mt-3 pt-3 border-t border-[var(--tailwind-colors-rdns-700)]">
            <ol className="mt-2 list-decimal pl-5 space-y-2 text-sm text-[var(--tailwind-colors-slate-200)]">
                <li>Download the configuration profile.</li>
                <li>Open the downloaded .mobileconfig file.</li>
                <li>Go to Apple menu &gt; System Settings (or System Preferences on older macOS).</li>
                <li>Navigate to General &gt; Device Management (or Privacy &amp; Security &gt; Profiles on older macOS).</li>
                <li>Select the profile and click Install.</li>
            </ol>
        </div>
    );
}
