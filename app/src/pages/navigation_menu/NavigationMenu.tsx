import { Button } from "@/components/ui/button";
import {
    GlobeIcon,
    ListIcon,
    SettingsIcon,
    ShieldIcon,
    FilterX,
    UserIcon,
    LogOut,
    Mail,
    HelpCircle,
    X,
    Sun,
    Moon,
} from "lucide-react";
import { useTheme } from "@/components/theme-provider";
import modDNSLogoDarkTheme from '@/assets/logos/modDNS-dark-theme.svg'
import modDNSLogoLightTheme from '@/assets/logos/modDNS-light-theme.svg'
import modDNSLogoCollapsedWhite from '@/assets/logos/o_white_250.png';
import modDNSLogoCollapsedBlack from '@/assets/logos/o_black_250.png';
import { type JSX, useContext, useState } from "react";
import { useLocation, useNavigate } from 'react-router-dom';
import { useNavigationCollapse } from "@/context/NavigationCollapseContext";
import { AuthContext } from "@/App";
import LogoutConfirmDialog from "@/components/dialogs/LogoutConfirmDialog";
import api from "@/api/api";

interface NavigationSectionProps {
    isMobile?: boolean;
    onClose?: () => void;
    offsetLeft?: number;
}

export default function NavigationSection({ isMobile = false, onClose, offsetLeft = 0 }: NavigationSectionProps): JSX.Element {
    const { collapsed } = useNavigationCollapse();
    const navigate = useNavigate();
    const auth = useContext(AuthContext);
    const [loading, setLoading] = useState(false);
    const [showLogoutDialog, setShowLogoutDialog] = useState(false);
    const { theme, setTheme } = useTheme();
    const isDarkMode = theme === 'dark';

    // Toggle between light and dark themes
    const toggleTheme = () => {
        setTheme(theme === 'dark' ? 'light' : 'dark');
    };

    // Get current theme icon
    const getThemeIcon = () => {
        return theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />;
    };

    // Logout logic
    const handleLogout = async () => {
        setLoading(true);

        try {
            auth?.logout?.();
            // Call the backend logout endpoint using API client
            await api.Client.authApi.apiV1AccountsLogoutPost();
        } catch (err: unknown) {
            console.error('Logout failed:', err);
        } finally {
            setLoading(false);
            setShowLogoutDialog(false);
            // Close mobile nav after logout
            if (isMobile && onClose) {
                onClose();
            }
        }
    };

    // Handle navigation for mobile
    const handleNavigation = (route: string) => {
        navigate(route);
        // Close mobile nav after navigation
        if (isMobile && onClose) {
            onClose();
        }
    };

    // Navigation menu items data
    const menuItems = [
        {
            icon: <GlobeIcon className="w-5 h-5" />,
            label: "DNS Setup",
            route: "/setup",
        },
        {
            icon: <ShieldIcon className="w-5 h-5" />,
            label: "Blocklists",
            route: "/blocklists",
        },
        {
            icon: <FilterX className="w-5 h-5" />,
            label: "Custom rules",
            route: "/custom-rules",
        },
        {
            icon: <ListIcon className="w-5 h-5" />,
            label: "Logs",
            route: "/query-logs",
        },
        {
            icon: <SettingsIcon className="w-5 h-5" />,
            label: "Settings",
            route: "/settings",
        },
        {
            icon: <UserIcon className="w-5 h-5" />,
            label: "Account",
            route: "/account-preferences",
        },
    ];

    const location = useLocation();

    // Sidebar width and style based on collapsed state or mobile
    const sidebarWidth = isMobile ? "w-full" : (collapsed ? "w-[64px]" : "w-[220px]");
    const logoSize = isMobile ? "w-[119px] h-7" : (collapsed ? "w-8 h-8" : "w-[119px] h-7");
    const showLabels = isMobile || !collapsed;

    // Helper to determine if a menu item is active
    const isActive = (route: string) => {
        // Exact match or startsWith for subroutes
        return location.pathname === route || location.pathname.startsWith(route + "/");
    };

    return (
        <aside
            role="navigation"
            data-testid={isMobile ? 'overlay-navigation' : 'main-navigation'}
            aria-label="Primary"
            /* Mobile menu: ensure scrollable in landscape by using dynamic viewport height and enabling vertical overflow. */
            className={`${isMobile ? 'relative w-full' : `fixed top-0 left-0 ${sidebarWidth}`} bg-[var(--sidebar-background)] ${isMobile ? '' : 'h-screen border-r border-border'} flex flex-col justify-between p-2 transition-all duration-200 ${isMobile ? 'z-auto overflow-y-auto overscroll-contain' : 'z-10'}`}
            style={isMobile ? { height: '100dvh', maxHeight: '100dvh' } : { minWidth: collapsed ? 64 : 220, left: offsetLeft }}
        >
            <div className="flex flex-col h-full justify-between">
                <div className="flex flex-col gap-6">
                    {/* Mobile header with close button */}
                    {isMobile && (
                        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--sidebar-border)]">
                            <div className="flex items-center space-x-3">
                                {/* Use theme-aware logo */}
                                <img src={isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme} alt="modDNS" className={logoSize} />
                            </div>
                            <button
                                onClick={onClose}
                                data-testid="nav-close"
                                className="p-2 hover:bg-[var(--tailwind-colors-rdns-alpha-900)] rounded-lg transition-colors"
                            >
                                <X className="h-6 w-6 text-[var(--sidebar-foreground)]" />
                            </button>
                        </div>
                    )}

                    {/* Logo for desktop */}
                    {!isMobile && (
                        <div className={`flex ${collapsed ? "flex-col items-center gap-2" : "items-center"} ${collapsed ? "px-0" : "px-2"}`}>
                            <div
                                className={`flex items-center cursor-pointer min-h-10 rounded-md px-2 py-2 transition-colors hover:bg-[var(--sidebar-muted)] ${showLabels ? "gap-2.5" : "justify-center"}`}
                                onClick={() => navigate("/home")}
                            >
                                {/* Use theme-aware logo */}
                                <img
                                    className={logoSize}
                                    alt="modDNS logo"
                                    src={collapsed ? (isDarkMode ? modDNSLogoCollapsedWhite : modDNSLogoCollapsedBlack) : (isDarkMode ? modDNSLogoDarkTheme : modDNSLogoLightTheme)}
                                />
                            </div>
                        </div>
                    )}

                    {/* Navigation Menu */}
                    <div className={`flex flex-col items-start gap-2 w-full ${isMobile ? 'px-2' : ''}`}>
                        {menuItems.map((item, index) => (
                            <Button
                                key={index}
                                variant="ghost"
                                className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full justify-start gap-2 rounded-md px-2 py-2 transition-colors focus:outline-none focus:ring-0 ${isActive(item.route) ? "bg-[var(--sidebar-accent-bg)] text-[var(--tailwind-colors-rdns-600)]" : "text-[var(--sidebar-foreground)] hover:bg-[var(--sidebar-muted)]"} ${!isMobile && collapsed ? "justify-center px-0" : "px-4"}`}
                                title={!isMobile && collapsed ? item.label : undefined}
                                onClick={() => handleNavigation(item.route)}
                            >
                                <span className={`flex items-center ${isActive(item.route) ? "text-[var(--tailwind-colors-rdns-600)]" : "text-[var(--sidebar-foreground)]"}`}>{item.icon}</span>
                                {showLabels && (
                                    <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'} ${isActive(item.route) ? "text-[var(--tailwind-colors-rdns-600)]" : "text-[var(--sidebar-foreground)]"}`}>
                                        {item.label}
                                    </span>
                                )}
                            </Button>
                        ))}
                    </div>
                </div>

                <div className="flex-1" />

                {/* Support Section */}
                <div className={`flex flex-col gap-2 ${isMobile ? 'px-2 border-t border-[var(--sidebar-border)] pt-4' : 'mb-4'}`}>
                    {/* FAQ */}
                    <Button
                        variant="ghost"
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full gap-2 rounded-md px-2 py-2 transition-colors text-[var(--sidebar-foreground)] hover:bg-[var(--sidebar-muted)] ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title="FAQ"
                        onClick={() => handleNavigation('/faq')}
                    >
                        <span className="flex items-center text-[var(--sidebar-foreground)]">
                            <HelpCircle className="w-5 h-5" />
                        </span>
                        {showLabels && (
                            <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'}`}>
                                FAQ
                            </span>
                        )}
                    </Button>

                    {/* Email Support */}
                    <Button
                        variant="ghost"
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full gap-2 rounded-md px-2 py-2 transition-colors text-[var(--sidebar-foreground)] hover:bg-[var(--sidebar-muted)] ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title="mailto:moddns@ivpn.net"
                        data-testid="nav-support"
                        onClick={() => {
                            window.open('mailto:moddns@ivpn.net', '_blank');
                            if (isMobile && onClose) onClose();
                        }}
                    >
                        <span className="flex items-center text-[var(--sidebar-foreground)]">
                            <Mail className="w-5 h-5" />
                        </span>
                        {showLabels && (
                            <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'}`}>
                                Support
                            </span>
                        )}
                    </Button>

                </div>

                {/* {showLabels && !isMobile && (
                    <div className="px-4">
                        <div className="h-px bg-[var(--sidebar-border)] w-full" />
                    </div>
                )} */}

                {/* Logout Button and Theme Toggle */}
                <div className={`relative flex items-center gap-2 ${isMobile ? 'px-2 pt-4' : ''}`}>
                    <Button
                        variant="ghost"
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} flex-1 gap-2 rounded-md px-2 py-2 transition-colors text-[var(--sidebar-foreground)] hover:bg-[var(--sidebar-muted)] ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title={!isMobile && collapsed ? "Logout" : undefined}
                        data-testid="btn-nav-logout"
                        onClick={() => setShowLogoutDialog(true)}
                        disabled={loading}
                    >
                        <span className="flex items-center text-[var(--sidebar-foreground)]">
                            <LogOut className="w-5 h-5" />
                        </span>
                        {showLabels && (
                            <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'} text-[var(--sidebar-foreground)]`}>
                                Log out
                            </span>
                        )}
                    </Button>
                    {/* Theme Toggle */}
                    <Button
                        variant="ghost"
                        size="icon"
                        className={`${isMobile ? 'h-12 w-12' : 'h-10 w-10'} rounded-md transition-colors text-[var(--sidebar-foreground)] hover:bg-[var(--sidebar-muted)] flex-shrink-0`}
                        title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                        onClick={toggleTheme}
                    >
                        {getThemeIcon()}
                    </Button>
                </div>
            </div>

            {/* Logout Confirmation Dialog */}
            <LogoutConfirmDialog
                open={showLogoutDialog}
                onOpenChange={setShowLogoutDialog}
                onConfirm={handleLogout}
                loading={loading}
            />
        </aside>
    );
}
