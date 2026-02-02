import { type Page, type Route } from '@playwright/test';
import { createMockAccount, createMockProfiles, createMockBlocklists } from './apiMocks';
import type { ModelAccount, ModelProfile } from '@/api/client/api';
import { AUTH_KEY } from '@/lib/consts';

export interface RegisterMocksOptions {
  authenticated?: boolean;
  profilesCount?: number;
  enableBlocklists?: string[];
  customProfiles?: Partial<ModelProfile>[]; // allow tests to pass partial override objects
  accountOverride?: Partial<ReturnType<typeof createMockAccount>>;
  extraRoutes?: (page: Page) => Promise<void> | void;
  ensureActiveProfile?: boolean; // new option
}

/**
 * Unified test mock registration.
 * - Seeds auth localStorage when authenticated
 * - Stubs core API endpoints (account, profiles, blocklists, custom rules, logs)
 * - Provides catch-all to avoid network leakage during deterministic tests
 */
export async function registerMocks(page: Page, opts: RegisterMocksOptions = {}) {
  const {
    authenticated = true,
    profilesCount = 2,
    enableBlocklists = ['bl-basic'],
    customProfiles,
    accountOverride,
    extraRoutes,
    ensureActiveProfile = true
  } = opts;

  const baseAccount: ModelAccount = createMockAccount();
  const account: ModelAccount = { ...baseAccount, ...accountOverride };
  const profiles: ModelProfile[] = customProfiles || createMockProfiles(profilesCount, {}, enableBlocklists);
  const blocklists = createMockBlocklists();

  if (authenticated) {
    await page.addInitScript((key) => { localStorage.setItem(key as string, 'true'); }, AUTH_KEY);
  } else {
    await page.addInitScript((key) => { localStorage.removeItem(key as string); }, AUTH_KEY);
  }

  // Core endpoints
  // Logout endpoint should be available regardless of authenticated state so user can always attempt logout
  await page.route(/http?:\/\/[^\s]+\/api\/v1\/accounts\/logout(\/|\?|$)|\/api\/v1\/accounts\/logout(\/|\?|$)/i, (r: Route) => {
    const method = r.request().method();
    // Debug log for visibility during test runs (non-fatal)
    console.log('[MOCK] intercept logout', method, r.request().url());
    if (method === 'OPTIONS') {
      return r.fulfill({ status: 200, headers: { 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Methods': 'POST, OPTIONS' }, body: '' });
    }
    if (method === 'POST') {
      return r.fulfill({ status: 200, contentType: 'application/json'});
    }
    return r.continue();
  });
  await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, (r: Route) => r.fulfill({
    status: authenticated ? 200 : 401,
    contentType: 'application/json',
    body: authenticated ? JSON.stringify(account) : '{}'
  }));

  await page.route(/\/api\/v1\/profiles(\/?|\?.*)$/i, (r: Route) => r.fulfill({
    status: authenticated ? 200 : 401,
    contentType: 'application/json',
    body: authenticated ? JSON.stringify(profiles) : '[]'
  }));

  if (authenticated) {
    await page.route(/\/api\/v1\/blocklists(\/?|\?.*)$/i, (r: Route) => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(blocklists) }));
    await page.route(/\/api\/v1\/custom-rules/i, (r: Route) => r.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
    await page.route(/\/api\/v1\/logs/i, (r: Route) => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], next: null }) }));
  }

  // Ensure active profile is set in store if applicable
  if (ensureActiveProfile && authenticated) {
    await page.addInitScript((p) => {
      try {
        const parsed = JSON.parse(p as string);
        // Zustand store exposed? If not, try window.__APP_STORE__ pattern guard
        // @ts-expect-error - accessing test instrumentation on window
        const store = window.__APP_STORE__ || undefined;
        if (store?.setActiveProfile) {
          store.setActiveProfile(parsed[0] || null);
        } else {
          // Fallback: stash for app bootstrap to consume if instrumentation added later
          localStorage.setItem('__test_active_profile__', parsed[0]?.profile_id || '');
        }
      } catch { /* ignore */ }
    }, JSON.stringify(profiles));
  }

  // Optional additional route registrations from caller
  if (extraRoutes) {
    await extraRoutes(page);
  }

  // Catch-all safety net
  await page.route(/\/api\/v1\//i, (r: Route) => {
    if (r.request().method() === 'GET') {
      if (authenticated) {
        const url = r.request().url();
        if (/\/api\/v1\/profiles/i.test(url)) {
          return r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(profiles) });
        }
        if (/\/api\/v1\/blocklists/i.test(url)) {
          return r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(blocklists) });
        }
        return r.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      }
      return r.fulfill({ status: 401, contentType: 'application/json', body: '{}' });
    }
    return r.continue();
  });
}
