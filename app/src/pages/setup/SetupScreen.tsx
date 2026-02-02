import { type JSX, useEffect, useRef, useState } from "react";
import { useScreenDetector } from '@/hooks/useScreenDetector';
import { MobileConnectionStatusBar } from '@/pages/setup/MobileConnectionStatusBar';
import { useLocation, useNavigate } from "react-router-dom";
import type { ModelProfile } from "@/api/client/api";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
    AppWindow,
    Clipboard,
    Gamepad2,
    Router,
    Smartphone,
    Tv2,
} from "lucide-react";
import AppleLogo from "@/assets/platforms/apple.svg";
import LinuxLogo from "@/assets/platforms/linux.svg";
import WindowsIcon from "@/assets/platforms/windows.svg";
import { useAppStore, useProfileData } from "@/store/general";
import VerificationBanner from '@/pages/setup/VerificationBanner';
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg';
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg';
import { useTheme } from "@/components/theme-provider";
import SetupGuidePanel from './RightPanelGuide';


interface SetupProps {
    profiles: ModelProfile[];
}

interface PlatformCard {
    icon: JSX.Element;
    name: string;
    disabled?: boolean;
}

const platformCards: PlatformCard[][] = [
    [
        { icon: <img src={WindowsIcon} alt="Windows Icon" className="w-6 h-6 brightness-0 dark:invert" />, name: "Windows" },
        { icon: <img src={AppleLogo} alt="Apple Icon" className="w-6 h-6 brightness-0 dark:invert" />, name: "macOS" },
        { icon: <img src={LinuxLogo} alt="Linux Icon" className="w-6 h-6 brightness-0 dark:invert" />, name: "Linux" },
    ],
    [
        { icon: <AppWindow className="w-6 h-6" />, name: "Browsers" },
        { icon: <Smartphone className="w-6 h-6" />, name: "Android" },
        { icon: <img src={AppleLogo} alt="Apple Icon" className="w-6 h-6 brightness-0 dark:invert" />, name: "iOS" },
    ],
    [
        { icon: <Router className="w-6 h-6" />, name: "Routers" },
        { icon: <Gamepad2 className="w-6 h-6" />, name: "Console", disabled: true },
        { icon: <Tv2 className="w-6 h-6" />, name: "Smart TV", disabled: true },
    ],
];

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export default function Setup({ profiles }: SetupProps): JSX.Element {
    const { isDesktop } = useScreenDetector();
    const setRightPanelOpen = useAppStore((state) => state.setRightPanelOpen);
    const account = useAppStore(state => state.account);
    const emailVerified = account?.email_verified;
    const [copiedField, setCopiedField] = useState<string | null>(null);
    const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
    const [showPanel, setShowPanel] = useState(false);
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);

    const location = useLocation();
    const navigate = useNavigate();
    const profileDeleted = location.state?.profileDeleted;
    const toastShown = useRef(false);

    useEffect(() => {
        if (profileDeleted && !toastShown.current) {
            toastShown.current = true;
            toast.success("Profile deleted.");
            // Clear state so toast doesn't show again
            navigate(location.pathname, { replace: true });
        }
    }, [profileDeleted, navigate, location.pathname]);

    // Auto-open right panel guide only when navigating back from Mobileconfig (explicit flag)
    useEffect(() => {
        const state = location.state as { fromMobileconfig?: boolean; platform?: string } | null;
        if (state?.fromMobileconfig && state.platform && !showPanel) {
            setSelectedPlatform(state.platform);
            setShowPanel(true);
            setRightPanelOpen(true);
            // Clear router state so closing the panel doesn't immediately reopen it
            navigate(location.pathname, { replace: true });
        }
    }, [location, showPanel, setRightPanelOpen, navigate]);

    // Copy to clipboard functionality
    const copyToClipboard = async (text: string, fieldName: string) => {
        try {
            await navigator.clipboard.writeText(text);
            setCopiedField(fieldName);
            setTimeout(() => setCopiedField(null), 2000);
            toast.success(`${fieldName} copied to clipboard`);
        } catch {
            toast.error("Failed to copy to clipboard");
        }
    };

    // Handle platform card click
    const handlePlatformClick = (platformName: string) => {
        setSelectedPlatform(platformName);
        setShowPanel(true);
        setRightPanelOpen(true);
    };

    // Handle guide panel close
    const handleGuideClose = () => {
        setShowPanel(false);
        setRightPanelOpen(false);
        // Wait for animation to complete before clearing selected platform
        setTimeout(() => {
            setSelectedPlatform(null);
        }, 500);
    };

    // Centralized derived profile data
    const profileData = useProfileData();

    return (
        <div className="flex flex-col w-full min-h-screen">
            {/* Main Content */}
            <div className="flex flex-1">
                {/* Content wrapper fills available (already reduced) width from ProtectedLayout and centers inner column */}
                <div className={`flex flex-col w-full p-6 min-h-screen ${!isDesktop && showPanel ? 'pointer-events-none opacity-0 select-none' : ''}`}>
                    <div data-testid="setup-container" className="max-w-[960px] mx-auto flex flex-col items-center w-full transition-all duration-500">
                        {/* Setup Content */}
                        <div className="flex flex-col items-center justify-center gap-8 w-full flex-1">
                            <div className="flex flex-col items-center gap-8 w-full">
                                <div className="flex flex-col items-center justify-center gap-4 w-full">
                                    <section className="flex flex-col items-center justify-center gap-8 w-full">
                                        <div className="flex flex-col max-w-[647px] items-center gap-6">
                                            <h1 className="text-3xl font-bold text-[var(--shadcn-ui-app-foreground)] text-center tracking-[-0.60px] font-mono">
                                                Setup
                                                <img
                                                    className="inline w-[240px] h-10 ml-2"
                                                    alt="modDNS logo"
                                                    src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                                                    style={{ verticalAlign: 'calc(1ex - 29px)' }}
                                                />
                                            </h1>

                                            <p className="max-w-[550px] text-lg text-[var(--shadcn-ui-app-muted-foreground)] text-center leading-8">
                                                Use the account-specific information and select from the list
                                                of guides below to set up modDNS on your device.
                                            </p>
                                            {/* Email verification warning banner (only if not verified & not dismissed) */}
                                            {account && !emailVerified && (
                                                <VerificationBanner emailVerified={emailVerified} />
                                            )}
                                        </div>

                                        {!isDesktop && profileData && (
                                            <div className="w-full max-w-[630px]">
                                                <MobileConnectionStatusBar />
                                            </div>
                                        )}

                                        {profileData && (
                                            <Card className="w-full max-w-[630px] rounded-md overflow-hidden border-none">
                                                <CardContent className="p-4">
                                                    <div className="flex flex-col items-start gap-3 w-full">
                                                        {Object.entries({
                                                            "Profile ID": profileData.id,
                                                            "DNS-over-TLS/QUIC": profileData.dnsOverTLS,
                                                            "DNS-over-HTTPS": profileData.dnsOverHTTPS,
                                                            "IPv4": profileData.ipv4,
                                                            // "IPv6": profileData.ipv6, // TODO: bring back when we have IPv6 support
                                                        }).map(([label, value], index) => {
                                                            const interactive = !isDesktop; // mobile & tablet
                                                            return (
                                                                <div
                                                                    key={index}
                                                                    className={`flex items-center justify-between w-full ${interactive ? 'cursor-pointer rounded-md px-2 -mx-2 active:bg-muted focus:bg-muted focus:outline-none' : ''}`}
                                                                    onClick={interactive ? () => copyToClipboard(value as string, label) : undefined}
                                                                    role={interactive ? 'button' : undefined}
                                                                    tabIndex={interactive ? 0 : undefined}
                                                                    aria-label={interactive ? `Copy ${label}` : undefined}
                                                                    onKeyDown={interactive ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); copyToClipboard(value as string, label); } } : undefined}
                                                                >
                                                                    <span className="text-sm text-[var(--shadcn-ui-app-muted-foreground)] leading-[25.4px] select-none">
                                                                        {label}
                                                                    </span>
                                                                    <div className="inline-flex items-center justify-end gap-2">
                                                                        <span className="text-sm text-[var(--shadcn-ui-app-foreground)] leading-5 font-mono break-all select-all">
                                                                            {value}
                                                                        </span>
                                                                        <Button
                                                                            variant="ghost"
                                                                            size="icon"
                                                                            className="p-0.5 h-auto bg-[var(--shadcn-ui-app-card)] text-[var(--tailwind-colors-rdns-600)] rounded-md hover:bg-[var(--tailwind-colors-rdns-600)] hover:text-primary-foreground"
                                                                            onClick={(e) => { e.stopPropagation(); copyToClipboard(value as string, label); }}
                                                                            disabled={copiedField === label}
                                                                            aria-label={`Copy ${label}`}
                                                                        >
                                                                            <Clipboard className="w-4 h-4" />
                                                                        </Button>
                                                                    </div>
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                </CardContent>
                                            </Card>
                                        )}
                                    </section>

                                    <section className="flex flex-col items-center gap-7 w-full">
                                        <div className="hidden lg:flex coarse:hidden flex-col w-full max-w-[630px] items-start gap-3" data-testid="setup-platforms-desktop">
                                            {platformCards.map((row, rowIndex) => (
                                                <div key={rowIndex} className="flex items-center gap-3 w-full">
                                                    {row.map((platform, platformIndex) => (
                                                        <Card
                                                            key={platformIndex}
                                                            data-testid={`setup-platform-card-desktop-${platform.name.replace(/\s+/g, '-').toLowerCase()}`}
                                                            className={`flex-1 rounded-md border-none transition-all duration-300 ${platform.disabled
                                                                ? 'opacity-0 pointer-events-none'
                                                                : `hover:scale-105 cursor-pointer transform ${selectedPlatform === platform.name
                                                                    ? 'bg-[var(--tailwind-colors-rdns-600)]'
                                                                    : 'bg-[var(--variable-collection-surface)] hover:bg-[var(--shadcn-ui-app-accent)]'
                                                                }`
                                                                }`}
                                                            onClick={platform.disabled ? undefined : () => handlePlatformClick(platform.name)}
                                                        >
                                                            <CardContent className="flex items-center justify-center gap-3">
                                                                <div className="text-[var(--shadcn-ui-app-foreground)]">
                                                                    {platform.icon}
                                                                </div>
                                                                <span className="text-sm text-[var(--shadcn-ui-app-foreground)] leading-5">
                                                                    {platform.name}
                                                                </span>
                                                            </CardContent>
                                                        </Card>
                                                    ))}
                                                </div>
                                            ))}
                                            {/* Device Identification Card - full width */}
                                            <Card
                                                data-testid="setup-platform-card-desktop-device-identification"
                                                className={`w-full rounded-md border-none hover:scale-105 transition-all duration-300 cursor-pointer transform ${selectedPlatform === 'Device Identification'
                                                    ? 'bg-[var(--tailwind-colors-rdns-600)] shadow-lg shadow-[var(--tailwind-colors-rdns-600)]/20'
                                                    : 'bg-[var(--variable-collection-surface)] hover:bg-[var(--shadcn-ui-app-accent)]'
                                                    }`}
                                                onClick={() => handlePlatformClick('Device Identification')}
                                            >
                                                <CardContent className="flex items-center gap-3 p-4">
                                                    <div className={`flex items-center justify-center w-8 h-8 rounded-md transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                        ? 'bg-[var(--shadcn-ui-app-background)] shadow-md'
                                                        : 'bg-[var(--tailwind-colors-rdns-600)]'
                                                        }`}>
                                                        <Smartphone className={`w-4 h-4 transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--tailwind-colors-rdns-600)]'
                                                            : 'text-white'
                                                            }`} />
                                                    </div>
                                                    <div className="flex-1">
                                                        <h3 className={`text-sm font-semibold transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--shadcn-ui-app-background)] font-bold'
                                                            : 'text-[var(--shadcn-ui-app-foreground)]'
                                                            }`}>
                                                            Device Identification
                                                        </h3>
                                                        <p className={`text-xs transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--shadcn-ui-app-background)] opacity-80'
                                                            : 'text-[var(--shadcn-ui-app-muted-foreground)]'
                                                            }`}>
                                                            Optional: Identify specific devices in your logs
                                                        </p>
                                                    </div>
                                                </CardContent>
                                            </Card>
                                        </div>

                                        {/* Mobile / Tablet (responsive grid) now always rendered, hidden on md+ */}
                                        {/* Mobile / tablet grid: base grid; hide on large fine-pointer (true desktop); re-show on coarse pointer even if wide. */}
                                        <div className="w-full max-w-[630px] grid grid-cols-2 xs:grid-cols-3 gap-3 lg:hidden coarse:grid" data-testid="setup-platforms-mobile">
                                            {platformCards.flat().filter(platform => !platform.disabled).map((platform, idx) => (
                                                <Card
                                                    key={idx}
                                                    data-testid={`setup-platform-card-${platform.name.replace(/\s+/g, '-').toLowerCase()}`}
                                                    className={`rounded-md border-none hover:scale-[1.03] active:scale-100 transition-all duration-300 cursor-pointer ${selectedPlatform === platform.name
                                                        ? 'bg-[var(--tailwind-colors-rdns-600)]'
                                                        : 'bg-[var(--variable-collection-surface)] hover:bg-[var(--shadcn-ui-app-accent)]'
                                                        }`}
                                                    onClick={() => handlePlatformClick(platform.name)}
                                                >
                                                    <CardContent className="flex flex-col items-center justify-center gap-2 py-4 px-2">
                                                        <div className="text-[var(--shadcn-ui-app-foreground)]">
                                                            {platform.icon}
                                                        </div>
                                                        <span className="text-xs sm:text-sm text-center text-[var(--shadcn-ui-app-foreground)] leading-4 sm:leading-5 truncate w-full">
                                                            {platform.name}
                                                        </span>
                                                    </CardContent>
                                                </Card>
                                            ))}
                                            <Card
                                                data-testid="setup-platform-card-device-identification"
                                                className={`col-span-2 xs:col-span-3 rounded-md border-none hover:scale-[1.02] transition-all duration-300 cursor-pointer ${selectedPlatform === 'Device Identification'
                                                    ? 'bg-[var(--tailwind-colors-rdns-600)] shadow-lg shadow-[var(--tailwind-colors-rdns-600)]/20'
                                                    : 'bg-[var(--variable-collection-surface)] hover:bg-[var(--shadcn-ui-app-accent)]'
                                                    }`}
                                                onClick={() => handlePlatformClick('Device Identification')}
                                            >
                                                <CardContent className="flex items-center gap-3 p-4">
                                                    <div className={`flex items-center justify-center w-8 h-8 rounded-md transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                        ? 'bg-[var(--shadcn-ui-app-background)] shadow-md'
                                                        : 'bg-[var(--tailwind-colors-rdns-600)]'
                                                        }`}>
                                                        <Smartphone className={`w-4 h-4 transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--tailwind-colors-rdns-600)]'
                                                            : 'text-white'
                                                            }`} />
                                                    </div>
                                                    <div className="flex-1 min-w-0">
                                                        <h3 className={`text-sm font-semibold transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--shadcn-ui-app-background)] font-bold'
                                                            : 'text-[var(--shadcn-ui-app-foreground)]'
                                                            }`}>
                                                            Device Identification
                                                        </h3>
                                                        <p className={`text-xs transition-all duration-300 ${selectedPlatform === 'Device Identification'
                                                            ? 'text-[var(--shadcn-ui-app-background)] opacity-80'
                                                            : 'text-[var(--shadcn-ui-app-muted-foreground)]'
                                                            }`}>
                                                            Optional: Identify specific devices in your logs
                                                        </p>
                                                    </div>
                                                </CardContent>
                                            </Card>
                                        </div>
                                    </section>
                                </div> {/* end inner gap wrap */}
                            </div> {/* end items-center gap wrapper */}
                        </div> {/* end flex-1 content column */}
                    </div> {/* end max-width center wrapper */}
                </div> {/* end content wrapper */}
            </div> {/* end flex-1 row */}

            {/* Right Panel - Instructions (absolutely positioned) */}
            {selectedPlatform && (
                <SetupGuidePanel
                    platform={selectedPlatform!}
                    onClose={handleGuideClose}
                    isVisible={showPanel}
                    // Force overlay for widths <=1024 to avoid header overlap issues on tablets / iPhone landscape
                    mode={isDesktop ? 'sidepanel' : 'overlay'}
                />
            )}
        </div>
    );
}
