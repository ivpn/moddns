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
} from "lucide-react";
import modDNSLogo from '@/assets/logos/modDNS.svg'
import modDNSLogoCollapsed from '@/assets/logos/o_white_250.png';
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

    // Logout logic
    const handleLogout = async () => {
        setLoading(true);

        try {
            auth?.logout?.();
            // Call the backend logout endpoint using API client
            await api.Client.authApi.apiV1AccountsLogoutPost();
        } catch (err: any) {
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
            className={`${isMobile ? 'relative w-full' : `fixed top-0 left-0 ${sidebarWidth}`} bg-[var(--shadcn-ui-app-background)] ${isMobile ? '' : 'h-screen border-r border-border'} flex flex-col justify-between p-2 transition-all duration-200 ${isMobile ? 'z-auto overflow-y-auto overscroll-contain' : 'z-10'}`}
            style={isMobile ? { height: '100dvh', maxHeight: '100dvh' } : { minWidth: collapsed ? 64 : 220, left: offsetLeft }}
        >
            <div className="flex flex-col h-full justify-between">
                <div className="flex flex-col gap-6">
                    {/* Mobile header with close button */}
                    {isMobile && (
                        <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--tailwind-colors-slate-600)]">
                            <div className="flex items-center space-x-3">
                                <img src={modDNSLogo} alt="modDNS" className={logoSize} />
                            </div>
                            <button
                                onClick={onClose}
                                data-testid="nav-close"
                                className="p-2 hover:bg-[var(--tailwind-colors-rdns-alpha-900)] rounded-lg transition-colors"
                            >
                                <X className="h-6 w-6 text-[var(--tailwind-colors-slate-50)]" />
                            </button>
                        </div>
                    )}

                    {/* Logo for desktop */}
                    {!isMobile && (
                        <div
                            className={`flex items-center cursor-pointer min-h-10 rounded-md px-2 py-2 transition-colors hover:bg-[var(--tailwind-colors-rdns-alpha-900)] ${showLabels ? "w-full gap-2.5 justify-start" : "justify-center"} ${collapsed ? "px-0" : "px-4"}`}
                            onClick={() => navigate("/home")}
                        >
                            <img
                                className={logoSize}
                                alt="modDNS logo"
                                src={collapsed ? modDNSLogoCollapsed : modDNSLogo}
                            />
                        </div>
                    )}

                    {/* Navigation Menu */}
                    <div className={`flex flex-col items-start gap-2 w-full ${isMobile ? 'px-2' : ''}`}>
                        {menuItems.map((item, index) => (
                            <Button
                                key={index}
                                variant="ghost"
                                className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full justify-start gap-2 rounded-md px-2 py-2 transition-colors focus:outline-none focus:ring-0 ${isActive(item.route) ? "bg-[var(--shadcn-ui-app-muted)] text-primary" : "text-[var(--tailwind-colors-slate-50)] hover:bg-[var(--shadcn-ui-app-muted)]"} ${!isMobile && collapsed ? "justify-center px-0" : "px-4"}`}
                                title={!isMobile && collapsed ? item.label : undefined}
                                onClick={() => handleNavigation(item.route)}
                            >
                                <span className={`flex items-center ${isActive(item.route) ? "text-[var(--tailwind-colors-rdns-600)]" : ""}`}>{item.icon}</span>
                                {showLabels && (
                                    <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'} ${isActive(item.route) ? "text-[var(--tailwind-colors-rdns-600)]" : "text-[var(--tailwind-colors-slate-50)]"}`}>
                                        {item.label}
                                    </span>
                                )}
                            </Button>
                        ))}
                    </div>
                </div>

                <div className="flex-1" />

                {/* Support Section */}
                <div className={`flex flex-col gap-2 ${isMobile ? 'px-2 border-t border-[var(--tailwind-colors-slate-600)] pt-4' : 'mb-4'}`}>
                    {/* {showLabels && !isMobile && (
                        <div className="px-4">
                            <div className="h-px bg-[var(--tailwind-colors-slate-600)] w-full" />
                        </div>
                    )} */}

                    {/* FAQ */}
                    <Button
                        variant="ghost"
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full gap-2 rounded-md px-2 py-2 transition-colors text-[var(--tailwind-colors-slate-50)] hover:bg-[var(--shadcn-ui-app-muted)] ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title="FAQ"
                        onClick={() => handleNavigation('/faq')}
                    >
                        <span className="flex items-center">
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
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full gap-2 rounded-md px-2 py-2 transition-colors text-[var(--tailwind-colors-slate-50)] hover:bg-[var(--shadcn-ui-app-muted)] ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title="mailto:moddns@ivpn.net"
                        data-testid="nav-support"
                        onClick={() => {
                            window.open('mailto:moddns@ivpn.net', '_blank');
                            if (isMobile && onClose) onClose();
                        }}
                    >
                        <span className="flex items-center">
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
                        <div className="h-px bg-[var(--tailwind-colors-slate-600)] w-full" />
                    </div>
                )} */}

                {/* Logout Button */}
                <div className={`relative ${isMobile ? 'px-2 pt-4' : ''}`}>
                    <Button
                        variant="ghost"
                        className={`flex ${isMobile ? 'min-h-12' : 'min-h-10'} w-full gap-2 rounded-md px-2 py-2 transition-colors hover:bg-[var(--destructive)]/10 ${!isMobile && collapsed ? "justify-center px-0" : "justify-start px-4"}`}
                        title={!isMobile && collapsed ? "Logout" : undefined}
                        data-testid="btn-nav-logout"
                        onClick={() => setShowLogoutDialog(true)}
                        disabled={loading}
                    >
                        <span className="flex items-center">
                            <LogOut className="w-5 h-5" />
                        </span>
                        {showLabels && (
                            <span className={`font-medium ${isMobile ? 'text-base' : 'text-sm'}`}>
                                Log out
                            </span>
                        )}
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
