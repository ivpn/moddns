import React, { lazy, Suspense, useState, useEffect, useRef, createContext, useContext, useCallback } from "react";
import { useHeaderStackHeight } from '@/lib/useHeaderStackHeight';
import NavigationMenu from './pages/navigation_menu/NavigationMenu';
import { useScreenDetector } from './hooks/useScreenDetector';
import Header from './pages/header/Header';
import BottomNav from './components/navigation/BottomNav';
import ConnectionStatusHeader from './pages/header/ConnectionStatusHeader';
import { NavigationCollapseProvider, useNavigationCollapse } from "@/context/NavigationCollapseContext";

// Lazy-loaded page components (route-level code splitting)
const Setup = lazy(() => import('./pages/setup/Setup'));
const Settings = lazy(() => import('./pages/settings/Settings'));
const PasswordReset = lazy(() => import('./pages/auth/PasswordReset'));
const PasswordResetConfirm = lazy(() => import('./pages/auth/PasswordResetConfirm'));
const Logs = lazy(() => import('./pages/logs/Logs'));
const Blocklists = lazy(() => import('./pages/blocklists/Blocklists'));
const CustomRules = lazy(() => import('./pages/custom_rules/CustomRules'));
const Login = lazy(() => import('./pages/auth/Login'));
const Signup = lazy(() => import('./pages/auth/Signup'));
const TermsOfService = lazy(() => import('./pages/legal/TermsOfService'));
const PrivacyPolicy = lazy(() => import("./pages/legal/PrivacyPolicy"));
const FAQ = lazy(() => import("./pages/legal/FAQ"));
const NotFound = lazy(() => import("./pages/NotFound"));
const AccountPreferences = lazy(() => import('@/pages/account_preferences/Account'));
const MobileconfigPage = lazy(() => import('@/pages/mobileconfig/MobileconfigPage'));
const MobileconfigDownload = lazy(() => import('@/pages/mobileconfig/MobileconfigDownload'));
const HomeScreen = lazy(() => import('./pages/home/HomeScreen'));

import { createBrowserRouter, RouterProvider, Navigate, Outlet, useLoaderData, useLocation, useNavigate, redirect } from 'react-router-dom';
import { ThemeProvider } from "@/components/theme-provider"
import api from "@/api/api";
import type { ModelAccount, ModelProfile } from "@/api/client/api";
import { AUTH_KEY } from "@/lib/consts"
import { useAppStore } from "@/store/general"
import { Toaster } from "@/components/ui/sonner"
import { ApiErrorBoundary } from "@/components/errors/ApiErrorBoundary";
import { RouterErrorBoundary } from "@/components/errors/RouterErrorBoundary";
import { useApiEventHandler } from "@/api/eventHandler";
import { toast } from "sonner";
import { authToasts } from "@/lib/authToasts";
import { subscribe, dispatch, type AppEvent } from '@/lib/eventBus';

// Desktop layout sizing constants
const DESKTOP_CONTENT_BASE_WIDTH = 1200;
const DESKTOP_CONTENT_MAX_WIDTH = 1360;
const DESKTOP_CONTENT_CLAMP = `clamp(${DESKTOP_CONTENT_BASE_WIDTH}px, 76vw, ${DESKTOP_CONTENT_MAX_WIDTH}px)`;
const ULTRAWIDE_CONTENT_MAX_WIDTH = DESKTOP_CONTENT_MAX_WIDTH;

// Auth context to manage authentication state
type AuthContextType = {
  isAuthenticated: boolean;
  login: (showToast?: boolean) => void;
  logout: (toastMessage?: string, toastType?: 'success' | 'info' | 'error' | 'warning') => void;
};
const AuthContext = createContext<AuthContextType | undefined>(undefined);

function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used within AuthProvider");
  return context;
}

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const navigate = useNavigate();
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    return localStorage.getItem(AUTH_KEY) === "true";
  });

  useEffect(() => {
    localStorage.setItem(AUTH_KEY, isAuthenticated ? "true" : "false");
  }, [isAuthenticated]);

  // Public route predicate (keep in sync with router public section)
  const isPublicPath = (p: string) => (
    p === '/login' ||
    // Dynamic signup route requires subid; plain /signup should not be treated as public and will fall through to 404
    p.startsWith('/signup/') ||
    p === '/tos' ||
    p === '/privacy' ||
    p === '/standalone-faq' ||
    p === '/reset-password' ||
    p.startsWith('/reset-password/') ||
    p.startsWith('/verify/email/') ||
    p.startsWith('/short/')
  );

  // Universal redirect safeguard when auth state flips to false, but allow public paths
  useEffect(() => {
    if (!isAuthenticated) {
      const current = window.location.pathname;
      if (!isPublicPath(current)) {
        navigate('/login', { replace: true });
      }
    }
  }, [isAuthenticated, navigate]);

  const login = (showToast: boolean = true) => {
    setIsAuthenticated(true);
    localStorage.setItem(AUTH_KEY, "true");
    if (showToast) authToasts.loginSuccess();
  };

  const performLogoutSideEffects = () => {
    localStorage.removeItem(AUTH_KEY);
    useAppStore.getState().setAccount(null);
    useAppStore.getState().setProfiles([]);
    useAppStore.getState().setActiveProfile(null);
  };

  const logout = (toastMessage?: string, toastType: 'success' | 'info' | 'error' | 'warning' = 'success') => {
    setIsAuthenticated(false);
    performLogoutSideEffects();
    if (toastMessage) {
      toast[toastType](toastMessage);
    } else {
      authToasts.logoutSuccess();
    }
  };

  // Subscribe to event bus for auth related forced logout events
  useEffect(() => {
    const unsub = subscribe((ev: AppEvent) => {
      if (ev.type === 'auth/forceLogout' || ev.type === 'auth/sessionExpired') {
        if (!isAuthenticated) return; // idempotent
        setIsAuthenticated(false);
        performLogoutSideEffects();
        const reason = ev.type === 'auth/sessionExpired' ? 'Session expired - please log in again.' : ev.reason;
        if (reason === 'Session expired - please log in again.') {
          authToasts.sessionExpired();
        } else if (reason) {
          toast[ev.type === 'auth/forceLogout' ? (ev.toastType || 'error') : 'error'](reason);
        } else {
          authToasts.logoutSuccess();
        }
        if (window.location.pathname !== '/login') {
          navigate('/login', { replace: true });
        }
      }
    });
    return () => { unsub(); };
  }, [isAuthenticated, navigate]);

  // Removed legacy __session_expired_flag__ flush (event bus handles timing via queue)

  return (
    <AuthContext.Provider value={{ isAuthenticated, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

// Loader for protected routes that need both account and profiles data
async function rootLoader() {
  try {
    // Check if user is authenticated before making API calls
    const authToken = localStorage.getItem(AUTH_KEY);
    if (!authToken || authToken !== "true") {
      // If no auth token, redirect to login instead of making API calls
      throw redirect("/login");
    }

    const [accountRes, profilesRes] = await Promise.all([
      api.Client.accountsApi.apiV1AccountsCurrentGet(),
      api.Client.profilesApi.apiV1ProfilesGet(),
    ]);

    // Save to Zustand store
    const account = accountRes.data as ModelAccount;
    const profiles = profilesRes.data as ModelProfile[];

    // This will run on the client, so we can update the store here:
    if (typeof window !== "undefined") {
      const { setAccount, setProfiles, restoreActiveProfile } = useAppStore.getState();
      setAccount(account);
      setProfiles(profiles);
      // Restore the previously selected profile or set the first one
      restoreActiveProfile(profiles);
    }

    return {
      account,
      profiles,
    };
  } catch (error: unknown) {




    if (error instanceof Response) throw error;
    const err = error as Record<string, unknown>;
    const status = (err?.response as Record<string, unknown>)?.status ?? err?.status ?? (error instanceof Error && (error as Record<string, unknown>).status);
    if (status === 401 || status === 404) {
      // Dispatch a unified session expired event; AuthProvider subscriber performs cleanup + toast
      if (typeof window !== 'undefined') {
        dispatch({ type: 'auth/sessionExpired' });
      }
      throw redirect('/login');
    }
    if (status === 429) {
      if (typeof window !== 'undefined') {
        setTimeout(() => toast.error('Too many requests. Some features may be temporarily unavailable.'), 100);
      }
      return { account: null, profiles: [] };
    }
    console.error('Root loader error (unhandled):', error);
    throw redirect('/login');
  }
}

// Lighter loader for pages that only need profiles data (no account data needed)
async function profilesOnlyLoader() {
  try {
    // Check if user is authenticated before making API calls
    const authToken = localStorage.getItem(AUTH_KEY);
    if (!authToken || authToken !== "true") {
      // If no auth token, redirect to login instead of making API calls
      throw redirect("/login");
    }

    const profilesRes = await api.Client.profilesApi.apiV1ProfilesGet();

    // Save to Zustand store
    const profiles = profilesRes.data as ModelProfile[];

    // This will run on the client, so we can update the store here:
    if (typeof window !== "undefined") {
      const { setProfiles, restoreActiveProfile } = useAppStore.getState();
      setProfiles(profiles);
      // Restore the previously selected profile or set the first one
      restoreActiveProfile(profiles);
    }

    return {
      account: null,
      profiles,
    };
  } catch (error: unknown) {




    if (error instanceof Response) throw error;
    const err = error as Record<string, unknown>;
    const status = (err?.response as Record<string, unknown>)?.status ?? err?.status ?? (error instanceof Error && (error as Record<string, unknown>).status);
    if (status === 401 || status === 404) {
      if (typeof window !== 'undefined') {
        dispatch({ type: 'auth/sessionExpired' });
      }
      throw redirect('/login');
    }
    if (status === 429) {
      if (typeof window !== 'undefined') {
        setTimeout(() => toast.error('Too many requests. Some features may be temporarily unavailable.'), 100);
      }
      return { account: null, profiles: [] };
    }
    console.error('Profiles loader error (unhandled):', error);
    throw redirect('/login');
  }
}

// Unified base layout for public/protected wrappers
function BaseLayout({ children, mode }: { children: React.ReactNode, mode: 'public' | 'app' }) {
  const baseClasses = 'relative flex flex-col min-h-screen overflow-x-hidden bg-[var(--shadcn-ui-app-background)]';
  if (mode === 'public') {
    return (
      <div data-testid="public-layout" className={baseClasses + ' w-full'} style={{ width: '100vw', maxWidth: '100vw' }}>
        {children}
      </div>
    );
  }
  return (
    <div className={'flex w-full min-h-screen overflow-x-hidden bg-[var(--shadcn-ui-app-background)]'}>
      {children}
    </div>
  );
}

// Backwards compatibility components (can be removed after updates)
const AppLayout = ({ children }: { children: React.ReactNode }) => <BaseLayout mode='app'>{children}</BaseLayout>;
const PublicLayout = ({ children }: { children: React.ReactNode }) => <BaseLayout mode='public'>{children}</BaseLayout>;

// Layout for protected routes
function ProtectedLayout() {
  const { isAuthenticated } = useAuth();
  const { collapsed } = useNavigationCollapse();
  const rightPanelOpen = useAppStore((state) => state.rightPanelOpen);
  const setRightPanelOpen = useAppStore((state) => state.setRightPanelOpen);
  const connectionStatusVisible = useAppStore((state) => state.connectionStatusVisible);
  const setConnectionStatusVisible = useAppStore((state) => state.setConnectionStatusVisible);
  const profiles = useAppStore((state) => state.profiles);
  const location = useLocation();
  const { isDesktop, navDesktop, width: viewportWidth } = useScreenDetector();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const handleMoreClick = useCallback(() => setMobileNavOpen(true), []);

  const connectionHeaderRef = useRef<HTMLDivElement | null>(null);
  const mainHeaderRef = useRef<HTMLDivElement | null>(null);
  useHeaderStackHeight([connectionHeaderRef, mainHeaderRef], { reducePx: 30 });

  useEffect(() => {
    if (rightPanelOpen && location.pathname !== '/setup') {
      setRightPanelOpen(false);
    }
  }, [location.pathname, rightPanelOpen, setRightPanelOpen]);

  const localAuthed = typeof window !== 'undefined' ? localStorage.getItem(AUTH_KEY) === 'true' : isAuthenticated;

  useEffect(() => {
    const revalidate = () => { };
    window.addEventListener('auth:logout', revalidate);
    window.addEventListener('storage', (e) => { if (e.key === AUTH_KEY) revalidate(); });
    return () => {
      window.removeEventListener('auth:logout', revalidate);
    };
  }, []);

  if (!isAuthenticated || !localAuthed) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (location.pathname === "/") {
    return <Navigate to="/home" replace />;
  }

  const showDialogTrigger = location.pathname === '/blocklists' || location.pathname === '/query-logs' || location.pathname === '/custom-rules';
  const showProfileDropdown = location.pathname !== '/home' && location.pathname !== '/account-preferences';
  const showLogoutButton = location.pathname === '/account-preferences';

  const getCurrentPageName = () => {
    switch (location.pathname) {
      case '/home':
        return '';
      case '/setup':
        return '';
      case '/blocklists':
        return 'Blocklists';
      case '/custom-rules':
        return 'Custom rules';
      case '/settings':
        return 'Settings';
      case '/query-logs':
        return 'Logs';
      case '/account-preferences':
        return 'Account preferences';
      case '/mobileconfig':
        return 'Mobile configuration';
      case '/faq':
        return 'FAQ';
      default:
        if (location.pathname.startsWith('/setup/')) return 'DNS Setup';
        if (location.pathname.startsWith('/blocklists/')) return 'Blocklists';
        if (location.pathname.startsWith('/custom-rules/')) return 'Custom rules';
        if (location.pathname.startsWith('/settings/')) return 'Settings';
        if (location.pathname.startsWith('/query-logs/')) return 'Logs';
        if (location.pathname.startsWith('/account-preferences/')) return 'Account';
        if (location.pathname.startsWith('/mobileconfig/')) return 'Mobile configuration';
        return 'Dashboard';
    }
  };

  const currentPageName = getCurrentPageName();
  const sidebarWidth = navDesktop ? (collapsed ? 64 : 220) : 0;
  const rightPanelWidth = 600;
  const headerRightOffset = rightPanelOpen ? rightPanelWidth : 0;
  const headerTopOffset = (isDesktop && connectionStatusVisible) ? 48 : 0;
  const shouldShowConnectionStatusRestore = isDesktop && !connectionStatusVisible;

  const shellOffset = isDesktop && viewportWidth >= 1400
    ? Math.max((viewportWidth - (sidebarWidth + ULTRAWIDE_CONTENT_MAX_WIDTH)) / 2, 0)
    : 0;

  const contentMaxWidth = isDesktop ? DESKTOP_CONTENT_CLAMP : '100%';

  return (
    <>
    <AppLayout>
      {navDesktop && <div data-testid="persistent-sidebar"><NavigationMenu offsetLeft={shellOffset} /></div>}

      {isDesktop && connectionStatusVisible && (
        <div
          ref={connectionHeaderRef}
          className="fixed top-0 right-0 z-50 transition-all duration-500"
          style={{ left: `${sidebarWidth + shellOffset}px`, right: `${headerRightOffset + shellOffset}px` }}
        >
          <div className="mx-auto w-full px-4 sm:px-6 lg:px-8" style={{ maxWidth: contentMaxWidth }}>
            <ConnectionStatusHeader />
          </div>
        </div>
      )}

      <div
        ref={mainHeaderRef}
        className={`fixed right-0 z-50 transition-all duration-500 ${isDesktop ? '' : 'left-0'}`}
        style={isDesktop ? {
          top: `${headerTopOffset}px`,
          left: `${sidebarWidth + shellOffset}px`,
          right: `${headerRightOffset + shellOffset}px`
        } : {
          top: '0px',
          left: '0px',
          right: '0px'
        }}
      >
        <div className="mx-auto w-full px-4 sm:px-6 lg:px-8" style={{ maxWidth: contentMaxWidth }}>
          <Header
            profiles={profiles || []}
            showProfileDropdown={showProfileDropdown}
            showLogoutButton={showLogoutButton}
            showDialogTrigger={showDialogTrigger}
            currentPageName={currentPageName}
            showConnectionStatusRestoreButton={shouldShowConnectionStatusRestore}
            onRestoreConnectionStatus={() => setConnectionStatusVisible(true)}
          />
        </div>
      </div>

      <div
        data-testid="app-content"
        className="transition-all duration-200 bg-[var(--shadcn-ui-app-background)] w-full overflow-x-hidden box-border"
        style={isDesktop ? {
          paddingTop: 'var(--app-header-stack, 64px)',
          marginLeft: `${sidebarWidth + shellOffset}px`,
          width: `calc(100vw - ${sidebarWidth + shellOffset}px - ${headerRightOffset + shellOffset}px)`,
          minHeight: 'calc(100vh - (var(--app-header-stack, 64px)))',
          maxWidth: '100vw'
        } : {
          paddingTop: 'var(--app-header-stack, 110px)',
          paddingBottom: '72px',
          paddingLeft: '0px',
          marginLeft: '0px',
          width: '100%',
          minHeight: 'calc(100dvh - 72px)',
          maxWidth: '100vw'
        }}
      >
        <div className="mx-auto w-full px-4 sm:px-6 lg:px-8" style={{ maxWidth: contentMaxWidth }}>
          <Outlet />
        </div>
      </div>

      {!navDesktop && <BottomNav onMoreClick={handleMoreClick} />}
    </AppLayout>

    {/* Mobile nav overlay – rendered outside AppLayout to avoid stacking context / overflow issues */}
    {!navDesktop && (
      <div className={`fixed inset-0 z-[100] ${mobileNavOpen ? '' : 'pointer-events-none'}`} data-testid="nav-overlay-wrapper">
        <div
          data-testid="nav-backdrop"
          className={`fixed inset-0 bg-black/50 cursor-pointer transition-opacity duration-300 ${mobileNavOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
          onClick={() => setMobileNavOpen(false)}
        />
        <div
          className={`fixed inset-y-0 left-0 w-[80%] max-w-[320px] bg-[var(--variable-collection-surface)] shadow-lg transition-transform duration-300 ${mobileNavOpen ? 'translate-x-0' : '-translate-x-full'}`}
          data-testid="nav-overlay-panel"
        >
          <NavigationMenu isMobile={true} onClose={() => setMobileNavOpen(false)} />
        </div>
      </div>
    )}
    </>
  );
}

function RootIndexRedirect() {
  const { isAuthenticated } = useAuth();
  const localAuthed = typeof window !== 'undefined' ? localStorage.getItem(AUTH_KEY) === 'true' : isAuthenticated;
  const target = isAuthenticated && localAuthed ? '/home' : '/login';

  return <Navigate to={target} replace />;
}

function SetupWithLoader() {
  const { account, profiles } = useLoaderData() as { account: ModelAccount | null, profiles: ModelProfile[] };
  return <Suspense fallback={<div />}><Setup account={account as ModelAccount} profiles={profiles} /></Suspense>;
}

function SettingsWithLoader() {
  const { profiles } = useLoaderData() as { account: ModelAccount | null, profiles: ModelProfile[] };
  return <Suspense fallback={<div />}><Settings profiles={profiles} /></Suspense>;
}

function BlocklistsWithLoader() {
  return <Suspense fallback={<div />}><Blocklists /></Suspense>;
}

function CustomRulesWithLoader() {
  const { profiles } = useLoaderData() as { account: ModelAccount | null, profiles: ModelProfile[] };
  return <Suspense fallback={<div />}><CustomRules profiles={profiles} /></Suspense>;
}

function AccountPreferencesWithLoader() {
  const { account } = useLoaderData() as { account: ModelAccount | null };
  return <Suspense fallback={<div />}><AccountPreferences account={account} /></Suspense>;
}

function MobileconfigWithLoader() {
  return <Suspense fallback={<div />}><MobileconfigPage /></Suspense>;
}

function QueryLogsWithLoader() {
  const { account, profiles } = useLoaderData() as { account: ModelAccount | null, profiles: ModelProfile[] };
  return <Suspense fallback={<div />}><Logs account={account as ModelAccount} profiles={profiles} /></Suspense>;
}

// LoginWrapper handles login and redirects after success
function LoginWrapper() {
  const { isAuthenticated } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const from = (location.state as Record<string, unknown>)?.from as Record<string, unknown> | undefined;
  const fromPath = from?.pathname as string || "/home";

  React.useEffect(() => {
    // Only redirect if user is authenticated AND they came from another protected page
    // Don't redirect if they're already on the login page (let Login component handle its own navigation)
    if (isAuthenticated && location.state?.from) {
      navigate("/home", { replace: true });
    }
  }, [isAuthenticated, navigate, fromPath, location.state]);

  // Always render the Login component to avoid black screen issues
  // The redirect will happen in useEffect when authentication state updates
  return <Suspense fallback={<div />}><Login /></Suspense>;
}

// Component that handles API events inside Router context
// Dedicated mount component so event handler has access to AuthContext (order matters)
function EventHandlerMount() {
  useApiEventHandler();
  return null;
}

function AppWithEventHandler() {
  return (
    <>
      <Toaster />
      <AuthProvider>
        <NavigationCollapseProvider>
          <EventHandlerMount />
          <Outlet />
        </NavigationCollapseProvider>
      </AuthProvider>
    </>
  );
}

// Define routes with proper separation
// Public routes are now grouped under a single PublicLayout parent to avoid remount flicker.
const router = createBrowserRouter([
  {
    path: "/",
    element: <AppWithEventHandler />,
    errorElement: <RouterErrorBoundary />,
    children: [
      { index: true, element: <RootIndexRedirect /> },
      // PUBLIC ROUTES (grouped under one persistent layout to reduce white flicker between transitions)
      {
        path: "",
        element: <PublicLayout><Outlet /></PublicLayout>,
        children: [
          { path: "login", element: <LoginWrapper /> },
          { path: "signup/:subid", element: <Suspense fallback={<div />}><Signup /></Suspense> },
          { path: "tos", element: <Suspense fallback={<div />}><TermsOfService /></Suspense> },
          { path: "privacy", element: <Suspense fallback={<div />}><PrivacyPolicy /></Suspense> },
          { path: "standalone-faq", element: <Suspense fallback={<div />}><FAQ /></Suspense> },
          { path: "reset-password", element: <Suspense fallback={<div />}><PasswordReset /></Suspense> },
          { path: "reset-password/:token", element: <Suspense fallback={<div />}><PasswordResetConfirm /></Suspense> },
          { path: "short/:code", element: <Suspense fallback={<div />}><MobileconfigDownload /></Suspense> },
        ]
      },

      // PROTECTED ROUTES (authentication required)
      {
        path: "/",
        element: <ProtectedLayout />,
        errorElement: <RouterErrorBoundary />, // Protected route errors
        children: [
          { loader: rootLoader, path: "home", element: <Suspense fallback={<div />}><HomeScreen /></Suspense> },
          { loader: rootLoader, path: "setup", element: <SetupWithLoader /> },
          { loader: rootLoader, path: "settings", element: <SettingsWithLoader /> },
          { loader: profilesOnlyLoader, path: "blocklists", element: <BlocklistsWithLoader /> },
          { loader: rootLoader, path: "custom-rules", element: <CustomRulesWithLoader /> },
          { loader: rootLoader, path: "account-preferences", element: <AccountPreferencesWithLoader /> },
          { loader: rootLoader, path: "mobileconfig", element: <MobileconfigWithLoader /> },
          { loader: rootLoader, path: "query-logs", element: <QueryLogsWithLoader /> },
          { path: "faq", element: <Suspense fallback={<div />}><FAQ /></Suspense> },
        ],
      },

      // 404 CATCH-ALL for any unmatched routes (within first-level children)
      { path: "*", element: <Suspense fallback={<div />}><NotFound /></Suspense> },
    ],
  },
  // Global catch-all (extra safety) - can be retained or removed
  { path: "*", element: <Suspense fallback={<div />}><NotFound /></Suspense> },
]);

function App() {
  // Note: data-shadcn-ui-mode attribute is now managed by ThemeProvider
  // to sync with the current theme selection

  return (
    <ApiErrorBoundary>
      <ThemeProvider defaultTheme="dark" storageKey="vite-ui-theme">
        <RouterProvider router={router} />
      </ThemeProvider>
    </ApiErrorBoundary>
  );
}


export default App;
// eslint-disable-next-line react-refresh/only-export-components
export { useAuth, AuthContext, RootIndexRedirect };
