import { Settings2, LogOutIcon, Menu } from "lucide-react";
import React, { useState, useContext, useEffect } from "react";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { Button } from "@/components/ui/button";
import type { ModelProfile } from "@/api/client/api";
import { useAppStore } from "@/store/general";
import ProfileDropdown from "@/pages/header/ProfileDropdown";
import BlocklistsPreferencesDialog from '@/pages/header/BlocklistsPreferencesDialog';
import LogoutConfirmDialog from "@/components/dialogs/LogoutConfirmDialog";
import { AuthContext } from "@/App";
import api from "@/api/api";
import { toast } from "sonner";
import { useNavigate, useLocation } from "react-router-dom";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg';
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg';
import { useTheme } from "@/components/theme-provider";
import NavigationSection from '@/pages/navigation_menu/NavigationMenu';
interface HeaderProps {
    showDialogTrigger?: boolean;
    profiles: ModelProfile[];
    showProfileDropdown?: boolean;
    showLogoutButton?: boolean;
    currentPageName?: string;
    showConnectionStatusRestoreButton?: boolean;
    onRestoreConnectionStatus?: () => void;
    mobileNavOpen?: boolean;
    setMobileNavOpen?: (open: boolean) => void;
}

export default function Header({
    showDialogTrigger = false,
    profiles,
    showProfileDropdown = true,
    showLogoutButton = false,
    currentPageName,
    showConnectionStatusRestoreButton = false,
    onRestoreConnectionStatus,
    mobileNavOpen: mobileNavOpenProp,
    setMobileNavOpen: setMobileNavOpenProp,
}: HeaderProps): React.JSX.Element {
    const { navDesktop } = useScreenDetector();
    const navigate = useNavigate();
    const location = useLocation();
    const currentProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);
    const setProfiles = useAppStore((state) => state.setProfiles);
    const auth = useContext(AuthContext);
    const [scrolled, setScrolled] = useState(false);
    const { theme } = useTheme();
    const isDarkMode = theme === 'dark';

    useEffect(() => {
        const onScroll = () => {
            const shouldBeScrolled = window.scrollY > 4;
            setScrolled((prev) => prev === shouldBeScrolled ? prev : shouldBeScrolled);
        };
        window.addEventListener('scroll', onScroll, { passive: true });
        onScroll();
        return () => window.removeEventListener('scroll', onScroll);
    }, []);

    // State to control BlocklistsPreferencesDialog open/close
    const [showBlocklistsDialog, setShowBlocklistsDialog] = useState(false);
    const [showLogoutDialog, setShowLogoutDialog] = useState(false);
    const [logoutLoading, setLogoutLoading] = useState(false);
    const [mobileNavOpenLocal, setMobileNavOpenLocal] = useState(false);
    const mobileNavOpen = mobileNavOpenProp ?? mobileNavOpenLocal;
    const setMobileNavOpen = setMobileNavOpenProp ?? setMobileNavOpenLocal;

    // Logout handler
    const handleLogout = async () => {
        setLogoutLoading(true);
        try {
            await api.Client.authApi.apiV1AccountsLogoutPost();
            auth?.logout?.();
        } catch {
            toast.error("Logout failed.");
        } finally {
            setLogoutLoading(false);
            setShowLogoutDialog(false);
        }
    };

    // Note: Active profile restoration is now handled by the store's restoreActiveProfile function
    // which is called from the rootLoader after profiles are loaded

    // Desktop header
    if (navDesktop) {
        return (
            <div className="flex items-center gap-6 px-8 py-4 bg-[var(--shadcn-ui-app-background)]">
                {/* Left: page name (sidebar auto-collapses based on width) */}
                <div className="flex items-center gap-3">
                    {currentPageName && (
                        <h2 className="font-bold text-[var(--tailwind-colors-slate-50)] text-2xl tracking-tight leading-8">
                            {currentPageName}
                        </h2>
                    )}
                </div>

                {/* Right: Profile dropdown/Logout button and Settings button */}
                <div className="ml-auto flex items-center gap-3 w-auto">
                    {showConnectionStatusRestoreButton && (
                        <Button
                            variant="secondary"
                            className="flex items-center gap-2 h-8 px-3 rounded-md border border-[var(--tailwind-colors-slate-700)] bg-[var(--shadcn-ui-app-background)] text-[var(--tailwind-colors-slate-50)] hover:bg-[var(--tailwind-colors-slate-900)]/60"
                            onClick={() => onRestoreConnectionStatus?.()}
                            data-testid="conn-header-show"
                            aria-label="Show DNS connection status"
                        >
                            <span className="text-[11px] font-semibold tracking-[0.08em]">DNS Status</span>
                        </Button>
                    )}
                    {showLogoutButton ? (
                        <Button
                            className="flex items-center gap-1 h-auto bg-[var(--tailwind-colors-slate-800)] hover:bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-800)]"
                            onClick={() => setShowLogoutDialog(true)}
                        >
                            <LogOutIcon className="w-4 h-4" />
                            <span className="text-xs">Logout</span>
                        </Button>
                    ) : showProfileDropdown ? (
                        <div className="flex flex-col items-start">
                            <ProfileDropdown
                                profiles={profiles}
                                currentProfile={currentProfile}
                                setActiveProfile={setActiveProfile}
                                setProfiles={setProfiles}
                            />
                        </div>
                    ) : null}
                    {showDialogTrigger && (
                        <>
                            <Button
                                variant="outline"
                                size="icon"
                                className="w-9 h-9 p-0 flex items-center justify-center bg-[var(--tailwind-colors-slate-800)] rounded-md border-0"
                                onClick={() => setShowBlocklistsDialog(true)}
                            >
                                <Settings2 className="h-4 w-4 text-[var(--tailwind-colors-rdns-600)]" />
                            </Button>
                            <BlocklistsPreferencesDialog currentProfile={currentProfile!} open={showBlocklistsDialog} onOpenChange={setShowBlocklistsDialog} />
                        </>
                    )}
                </div>

                {/* Logout Confirmation Dialog */}
                <LogoutConfirmDialog
                    open={showLogoutDialog}
                    onOpenChange={setShowLogoutDialog}
                    onConfirm={handleLogout}
                    loading={logoutLoading}
                />
            </div>
        );
    }

    // Mobile header
    return (
        <>
            <div data-testid="app-header-bar" data-slot="mobile-header" className={`flex items-center justify-between px-4 sm:px-6 py-4 bg-[var(--shadcn-ui-app-background)] transition-shadow duration-200 ${scrolled ? 'shadow-[0_2px_6px_-1px_rgba(0,0,0,0.5)]' : ''}`}>
                {/* Left: modDNS logo - hidden on /home page */}
                <div className="flex items-center min-w-0">
                    {location.pathname !== "/home" && (
                        <img
                            className="h-6 cursor-pointer flex-shrink-0"
                            alt="modDNS logo"
                            src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme}
                            onClick={() => navigate("/home")}
                        />
                    )}
                </div>
                {/* Right: profile dropdown + menu */}
                <div className="flex items-center gap-3 min-w-0 max-w-[70%] justify-end">
                    {showProfileDropdown && (
                        <div className="flex items-center min-w-0">
                            <ProfileDropdown
                                profiles={profiles}
                                currentProfile={currentProfile}
                                setActiveProfile={setActiveProfile}
                                setProfiles={setProfiles}
                            />
                        </div>
                    )}
                </div>
            </div>

            {/* Mobile page title row */}
            {currentPageName && (
                <div
                    data-testid="mobile-header-page-title"
                    className={`md:hidden px-4 sm:px-6 pt-1 pb-8 flex items-center justify-between gap-3 bg-[var(--shadcn-ui-app-background)] transition-shadow duration-200 ${scrolled ? 'shadow-[0_2px_6px_-1px_rgba(0,0,0,0.5)]' : ''}`}
                >
                    <h2 data-slot="mobile-page-title" className="font-bold text-[var(--tailwind-colors-slate-50)] text-3xl tracking-tight leading-8">
                        {currentPageName}
                    </h2>
                    {location.pathname === '/blocklists' && (
                        <Button
                            variant="outline"
                            size="icon"
                            aria-label="Open blocklists preferences"
                            className="w-11 h-11 min-h-11 p-0 flex items-center justify-center bg-[var(--tailwind-colors-slate-800)] border-0 rounded-lg"
                            onClick={() => setShowBlocklistsDialog(true)}
                        >
                            <Settings2 className="h-5 w-5 text-[var(--tailwind-colors-rdns-600)]" />
                        </Button>
                    )}
                </div>
            )}

            {/* Mobile Navigation */}
            {/* Mobile / tablet (incl. landscape tablets) overlay navigation; hidden for full navDesktop */}
            {!navDesktop && (
                <div className={`fixed inset-0 z-[100] ${mobileNavOpen ? '' : 'pointer-events-none'}`} data-testid="nav-overlay-wrapper">
                    {/* Backdrop */}
                    <div
                        data-testid="nav-backdrop"
                        className={`fixed inset-0 bg-black/50 cursor-pointer transition-opacity duration-300 ${mobileNavOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
                        onClick={() => setMobileNavOpen(false)}
                    />

                    {/* Navigation Panel */}
                    <div
                        className={`fixed inset-y-0 left-0 w-[80%] max-w-[320px] bg-[var(--variable-collection-surface)] shadow-lg transition-transform duration-300 ${mobileNavOpen ? 'translate-x-0' : '-translate-x-full'}`}
                        data-testid="nav-overlay-panel"
                    >
                        <NavigationSection isMobile={true} onClose={() => setMobileNavOpen(false)} />
                    </div>
                </div>
            )}

            {/* Settings Dialog for mobile */}
            {showDialogTrigger && (
                <BlocklistsPreferencesDialog
                    currentProfile={currentProfile!}
                    open={showBlocklistsDialog}
                    onOpenChange={setShowBlocklistsDialog}
                />
            )}
        </>
    );
}
