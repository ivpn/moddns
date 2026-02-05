import { type JSX } from "react";

// eslint-disable-next-line react-refresh/only-export-components
export const deviceIdentificationBadges = [
    { label: "Device Tracking" },
    { label: "DNS Logs" },
];

// eslint-disable-next-line react-refresh/only-export-components
export const createDeviceIdentificationSteps = (profileId = "your-profile-id", dnsOverHTTPS = "https://example.com/dns-query", domain = "example.com"): Array<{ instruction: JSX.Element }> => [
    {
        instruction: (
            <span>
                Device identification allows you to see which specific device made each DNS query in your logs, making it easier to track and troubleshoot DNS issues across multiple devices.
            </span>
        ),
    },
    {
        instruction: (
            <span>
                <strong>For DNS-over-HTTPS (DoH):</strong> Append the device name to the URL path (URL-encode if needed)
            </span>
        ),
    },
    {
        instruction: (
            <div className="space-y-2">
                <p>Profile only:</p>
                <div className="bg-[var(--tailwind-colors-slate-900)] rounded p-3 font-mono text-xs text-[var(--tailwind-colors-slate-50)]">
                    {dnsOverHTTPS}
                </div>
                <p>With device name:</p>
                <div className="bg-[var(--tailwind-colors-slate-900)] rounded p-3 font-mono text-xs text-[var(--tailwind-colors-slate-50)]">
                    {dnsOverHTTPS}/my-laptop<br />
                    {dnsOverHTTPS}/phone<br />
                    {dnsOverHTTPS}/John%27s%20iPhone
                </div>
                <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                    Note: Spaces and special characters must be URL-encoded (e.g., "John's iPhone" becomes "John%27s%20iPhone")
                </p>
            </div>
        ),
    },
    {
        instruction: (
            <span>
                <strong>For DNS-over-TLS/QUIC (DoT/DoQ):</strong> Prepend the device name to the domain (use "--" for spaces)
            </span>
        ),
    },
    {
        instruction: (
            <div className="space-y-2">
                <p>Profile only:</p>
                <div className="bg-[var(--tailwind-colors-slate-900)] rounded p-3 font-mono text-xs text-[var(--tailwind-colors-slate-50)]">
                    {profileId}.{domain}
                </div>
                <p>With device name:</p>
                <div className="bg-[var(--tailwind-colors-slate-900)] rounded p-3 font-mono text-xs text-[var(--tailwind-colors-slate-50)]">
                    my-laptop-{profileId}.{domain}<br />
                    home--router-{profileId}.{domain}<br />
                    john--s--iphone-{profileId}.{domain}
                </div>
                <p className="text-xs text-[var(--tailwind-colors-slate-400)]">
                    Note: Spaces become "--" and special characters are removed (e.g., "John's iPhone" becomes "john--s--iphone")
                </p>
            </div>
        ),
    },
    {
        instruction: (
            <div className="bg-[var(--tailwind-colors-rdns-950)] border border-[var(--tailwind-colors-rdns-800)] rounded p-3">
                <p className="text-xs text-[var(--tailwind-colors-rdns-300)] leading-relaxed">
                    <strong>Character Rules:</strong> Use letters (a–z, A–Z), digits (0–9), spaces, and hyphens. For DoH, device names must be URL-encoded when they contain spaces or special characters. Device names are automatically truncated to 16 characters and normalized for safety.
                </p>
            </div>
        ),
    },
];

// Default export for backwards compatibility
const deviceIdentificationSteps = createDeviceIdentificationSteps();
// eslint-disable-next-line react-refresh/only-export-components
export { deviceIdentificationSteps };

const DeviceIdentificationGuide = {
    badges: deviceIdentificationBadges,
    steps: deviceIdentificationSteps,
};

export default DeviceIdentificationGuide;
