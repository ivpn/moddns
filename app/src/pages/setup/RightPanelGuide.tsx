import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    X as XIcon,
    AppWindow,
    Smartphone,
    Router,
    Gamepad2,
    Tv2
} from "lucide-react";
import React, { type JSX, useLayoutEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAppStore } from "@/store/general";

// Import platform icons
import WindowsIcon from "@/assets/platforms/windows.svg";
import AppleLogo from "@/assets/platforms/apple.svg";
import LinuxLogo from "@/assets/platforms/linux.svg";

// Import guide data
import WindowsGuide, { createWindowsSteps } from "./guides/Windows";
import LinuxGuide, { createLinuxSteps } from "./guides/Linux";
import { useProfileData } from '@/store/general';
import { deviceIdentificationBadges, createDeviceIdentificationSteps } from "./guides/DeviceIdentification";
import BrowsersGuide, { createBrowsersSteps, browsersBadges } from "./guides/Browsers";
import { androidBadges, createAndroidSteps } from "./guides/Android";
import RoutersGuide, { createRoutersSteps } from "./guides/Routers";
import IOS from "./guides/AppleIOS";
import MacOS from "./guides/AppleMacOS";

const AndroidGuide = { badges: androidBadges };

const macOSBadges = [
    { label: "macOS" },
    { label: "DNS over HTTPS" },
    { label: "DNS over TLS" },
];

const iosBadges = [
    { label: "iOS" },
    { label: "iPadOS" },
    { label: "DNS over HTTPS" },
    { label: "DNS over TLS" },
];


interface SetupGuidePanelProps {
    platform: string;
    onClose: () => void;
    isVisible?: boolean;
    mode?: 'sidepanel' | 'overlay';
}

const platformIcons: { [key: string]: React.ReactNode } = {
    "Windows": <img src={WindowsIcon} alt="Windows" className="w-5 h-5 brightness-0 dark:invert" />,
    "macOS": <img src={AppleLogo} alt="macOS" className="w-5 h-5 brightness-0 dark:invert" />,
    "Linux": <img src={LinuxLogo} alt="Linux" className="w-5 h-5 brightness-0 dark:invert" />,
    "Browsers": <AppWindow className="w-5 h-5" />,
    "Android": <Smartphone className="w-5 h-5" />,
    "iOS": <img src={AppleLogo} alt="iOS" className="w-5 h-5 brightness-0 dark:invert" />,
    "Routers": <Router className="w-5 h-5" />,
    "Console": <Gamepad2 className="w-5 h-5" />,
    "Smart TV": <Tv2 className="w-5 h-5" />,
    "Device Identification": <Smartphone className="w-5 h-5" />,
};

const platformGuides: { [key: string]: { badges?: { label: string }[]; steps?: { instruction: React.ReactNode; step?: number }[] } | null } = {
    "Windows": WindowsGuide,
    "Linux": LinuxGuide,
    "Android": AndroidGuide,
    "macOS": { badges: macOSBadges },
    "iOS": { badges: iosBadges },
    "Routers": RoutersGuide,
    "Browsers": BrowsersGuide,
    "Device Identification": null, // Handle dynamically
};

export default function SetupGuidePanel({ platform, onClose, isVisible = true, mode = 'sidepanel' }: SetupGuidePanelProps): JSX.Element {
    const profileData = useProfileData();
    const effectivePrimaryIp = profileData?.ipv4 || '0.0.0.0';
    const effectiveDomain = profileData?.domain || 'example.com';
    const dohEndpoint = profileData?.dohEndpoint || 'https://example.com/dns-query/your-profile-id';
    // Handle Device Identification dynamically with actual values
    let guide;
    if (platform === "Device Identification") {
        guide = {
            badges: deviceIdentificationBadges,
            steps: createDeviceIdentificationSteps(
                profileData?.id || "your-profile-id",
                dohEndpoint,
                "example.com" // Default domain - could be extracted from dnsOverHTTPS
            )
        };
    } else if (platform === "Browsers") {
        guide = {
            badges: browsersBadges,
            steps: createBrowsersSteps({
                dohEndpoint
            })
        };
    } else if (platform === "Windows") {
        guide = {
            badges: WindowsGuide.badges,
            steps: createWindowsSteps({
                dohEndpoint,
                primaryIp: effectivePrimaryIp
            })
        };
    } else if (platform === "Linux") {
        guide = {
            badges: LinuxGuide.badges,
            steps: createLinuxSteps({
                profileId: profileData?.id || 'your-profile-id',
                primaryIp: effectivePrimaryIp,
                domain: effectiveDomain
            })
        };
    } else if (platform === "Android") {
        guide = {
            badges: androidBadges,
            steps: createAndroidSteps({
                dotEndpoint: profileData?.dnsOverTLS || 'your-profile-id.example.com'
            })
        };
    } else if (platform === "Routers") {
        guide = {
            badges: RoutersGuide.badges,
            steps: createRoutersSteps({
                dohEndpoint,
                anycastIpv4: effectivePrimaryIp,
                dnsServerDomain: effectiveDomain,
                dotHostname: profileData?.dnsOverTLS || `your-profile-id.${effectiveDomain}`
            })
        };
    } else {
        guide = platformGuides[platform] || WindowsGuide;
    }
    const icon = platformIcons[platform] || platformIcons["Windows"];
    const connectionStatusVisible = useAppStore((state) => state.connectionStatusVisible);
    const navigate = useNavigate();

    // Check if platform supports mobileconfig
    const supportsMobileconfig = platform === 'macOS' || platform === 'iOS';

    // Handle mobileconfig navigation
    const handleQuickSetup = () => {
        navigate('/mobileconfig', { state: { platform } });
    };

    // Dynamic positioning: respect the measured header stack height so our overlay starts BELOW the fixed header(s)
    // Header stack height is published to --app-header-stack by useHeaderStackHeight hook (App.tsx)
    // Fallbacks: mobile ~110px padding (App.tsx fallback) but actual visual header for /setup page is usually ~64-72px.
    const isMobile = typeof window !== 'undefined' ? window.innerWidth <= 768 : false;
    const baseTop = connectionStatusVisible ? 48 : 0;

    // Measure actual header + optional page title height + safe-area inset (iOS notch) for precise overlay offset.
    const [mobileTop, setMobileTop] = useState(0);
    useLayoutEffect(() => {
        if (!(mode === 'overlay')) return;
        const measure = () => {
            if (!isMobile) { setMobileTop(baseTop); return; }
            const header = document.querySelector('[data-testid=app-header-bar]') as HTMLElement | null;
            const title = document.querySelector('[data-testid=mobile-header-page-title]') as HTMLElement | null;
            let total = 0;
            if (header) total += header.getBoundingClientRect().height;
            if (title) total += title.getBoundingClientRect().height;
            // Add safe-area inset top if present
            const safe = parseInt(getComputedStyle(document.documentElement).getPropertyValue('env(safe-area-inset-top)').replace('px', '')) || 0;
            // Fallback if measurement fails
            if (total === 0) total = 64;
            setMobileTop(total + safe);
        };
        measure();
        const ro = new ResizeObserver(measure);
        const headerEl = document.querySelector('[data-testid=app-header-bar]');
        if (headerEl) ro.observe(headerEl);
        const titleEl = document.querySelector('[data-testid=mobile-header-page-title]');
        if (titleEl) ro.observe(titleEl);
        const mo = new MutationObserver(measure);
        mo.observe(document.body, { childList: true, subtree: true });
        window.addEventListener('orientationchange', measure);
        window.addEventListener('resize', measure);
        const id = setInterval(measure, 300);
        setTimeout(() => clearInterval(id), 1800);
        return () => {
            window.removeEventListener('orientationchange', measure);
            window.removeEventListener('resize', measure);
            ro.disconnect();
            mo.disconnect();
            clearInterval(id);
        };
    }, [mode, isMobile, baseTop]);

    const EXTRA_BUFFER = 24;
    const bufferedTop = mode === 'overlay' ? mobileTop + EXTRA_BUFFER : 0;
    const topOffsetValue = mode === 'overlay' ? `${bufferedTop}px` : '0';
    const height = mode === 'overlay' ? `calc(100dvh - ${bufferedTop}px)` : '100dvh';

    const isOverlay = mode === 'overlay';

    return (
        <div
            data-testid="setup-guide-panel"
            data-mode={isOverlay ? 'overlay' : 'sidepanel'}
            className={`fixed ${isOverlay ? 'inset-x-0' : 'right-0'} ${isOverlay ? 'w-full' : 'w-[600px]'} ${isOverlay ? 'rounded-none' : 'rounded-md'} bg-[var(--variable-collection-surface)] transition-all duration-500 ease-in-out z-40 ${isVisible
                ? 'transform translate-x-0 opacity-100'
                : 'transform translate-x-full opacity-0'
                }`}
            style={{
                top: topOffsetValue,
                height,
                maxHeight: height,
                overflow: 'hidden'
            }}
        >
            <div className="h-full relative flex flex-col">
                {/* Instructions Header */}
                <div className="flex items-center justify-between px-4 sm:px-6 h-[54px] sm:h-[62px] bg-[var(--variable-collection-surface)] border-b border-[var(--shadcn-ui-app-border)]" data-testid="setup-guide-header">
                    <div className="flex items-center gap-3 min-w-0">
                        {isOverlay && (
                            <Button variant="ghost" size="icon" onClick={onClose} className="h-6 w-6 shrink-0" data-testid="setup-guide-close-button">
                                <XIcon className="w-6 h-6 text-[var(--shadcn-ui-app-foreground)]" />
                            </Button>
                        )}
                        <div className="text-[var(--shadcn-ui-app-foreground)]">{icon}</div>
                        <div data-testid="setup-guide-title" className="text-sm sm:text-lg text-[var(--shadcn-ui-app-foreground)] leading-6 font-['Roboto_Flex-Regular',Helvetica] truncate">
                            {platform} setup
                        </div>
                    </div>
                    {!isOverlay && (
                        <Button variant="ghost" size="icon" onClick={onClose} className="h-6 w-6" data-testid="setup-guide-close-button">
                            <XIcon className="w-6 h-6 text-[var(--shadcn-ui-app-foreground)]" />
                        </Button>
                    )}
                </div>
                {/* Instructions Content */}
                <div className="p-4 sm:p-6 flex-1 min-h-0 overflow-y-auto overscroll-contain" data-testid="setup-guide-content">
                    <div className="flex flex-col gap-6">
                        {/* Tags */}
                        <div className="flex items-start gap-2.5 flex-wrap">
                            {guide.badges?.map((badge: { label: string }, index: number) => (
                                <Badge
                                    key={index}
                                    className="cursor-default select-none tracking-[0.08em] uppercase text-[10px] font-semibold px-3 py-1 rounded-[2px] border bg-[var(--variable-collection-surface)] text-[var(--shadcn-ui-app-foreground)] border-[var(--tailwind-colors-slate-400)] dark:border-[var(--tailwind-colors-slate-600)]"
                                >
                                    {badge.label}
                                </Badge>
                            ))}
                        </div>

                        {/* Quick Setup Button for Apple devices */}
                        {supportsMobileconfig && (
                            <div className="flex flex-col gap-3 p-4 bg-[var(--tailwind-colors-rdns-600)]/10 rounded-lg border border-[var(--tailwind-colors-rdns-600)]/30">
                                <p className="text-sm text-[var(--shadcn-ui-app-muted-foreground)]">
                                    Create a configuration profile to automatically apply our DNS settings. You will be prompted to download and install the file on your {platform} device.
                                </p>
                                <Button
                                    onClick={handleQuickSetup}
                                    className="w-full bg-[var(--tailwind-colors-rdns-600)] hover:bg-[var(--tailwind-colors-rdns-700)] text-white"
                                >
                                    Create Configuration Profile
                                </Button>
                                {platform === 'iOS' && <IOS />}
                                {platform === 'macOS' && <MacOS />}
                            </div>
                        )}

                        {/* Steps */}
                        <div className="flex flex-col gap-6">
                            {guide.steps?.map((step: { instruction: React.ReactNode; step?: number }, index: number) => (
                                <div key={index} className="flex flex-col gap-3">
                                    {step.step && (
                                        <div className="flex items-center gap-2.5">
                                            <div className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] leading-5 font-['Roboto_Flex-Regular',Helvetica]">
                                                STEP {step.step}
                                            </div>
                                        </div>
                                    )}
                                    <div className="text-sm text-[var(--shadcn-ui-app-foreground)] leading-6 font-['Roboto_Flex-Regular',Helvetica]">
                                        {step.instruction}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
