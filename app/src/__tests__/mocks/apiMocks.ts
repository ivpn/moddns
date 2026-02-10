import type { ModelAccount, ModelProfile, ModelProfileSettings, ModelAdvanced, ModelLogsSettings, ModelPrivacy, ModelSecurity, ModelStatisticsSettings } from '@/api/client/api';

// Minimal concrete sub-objects to satisfy required nested structures
const mockAdvanced = {
  // add required primitive properties if any appear later in schema
} as unknown as ModelAdvanced;
const mockLogs = {
  enabled: false,
  log_clients_ips: false,
  log_domains: false,
  retention: 0,
} as unknown as ModelLogsSettings;
const mockPrivacy = (enabledBlocklists: string[] = []): ModelPrivacy => ({
  default_rule: 'allow',
  blocklists_subdomains_rule: 'allow',
  blocklists: enabledBlocklists
} as unknown as ModelPrivacy);
const mockSecurity = {
  // fill with minimal required fields if present
} as unknown as ModelSecurity;
const mockStatistics = {
  enabled: false
} as unknown as ModelStatisticsSettings;

const baseProfileSettings = (profileId: string, enabledBlocklists: string[] = []): ModelProfileSettings => ({
  advanced: mockAdvanced,
  logs: mockLogs,
  privacy: mockPrivacy(enabledBlocklists),
  profile_id: profileId,
  security: mockSecurity,
  statistics: mockStatistics,
  custom_rules: []
});

export function createMockAccount(overrides: Partial<ModelAccount> = {}): ModelAccount {
  const base: ModelAccount = {
    id: 'mock-account-id',
    email: 'mock@example.com',
    email_verified: true,
    error_reports_consent: false,
    mfa: { totp: { enabled: false } },
    profiles: ['p1'],
    queries: 0,
    ...overrides
  };
  return base;
}

export function createMockProfiles(count = 1, overrides: Partial<ModelProfile> = {}, enabledBlocklists: string[] = []): ModelProfile[] {
  const list: ModelProfile[] = [];
  for (let i = 0; i < count; i++) {
    const id = `p${i + 1}`;
    const profile: ModelProfile = {
      account_id: 'mock-account-id',
      id,
      name: `Profile ${i + 1}`,
      profile_id: id,
      settings: baseProfileSettings(id, enabledBlocklists),
      ...overrides
    };
    list.push(profile);
  }
  return list;
}

// Convenience: build a small set of blocklists objects for UI listing
export function createMockBlocklists(): Record<string, unknown>[] {
  const now = new Date().toISOString();
  return [
    {
      blocklist_id: 'bl-basic',
      name: 'Basic Protection',
      description: 'Essential ads & trackers blocking.',
      entries: 12345,
      last_modified: now,
      homepage: 'https://example.com/basic',
      tags: ['basic']
    },
    {
      blocklist_id: 'bl-comprehensive',
      name: 'Comprehensive Shield',
      description: 'Broader set including malware & phishing.',
      entries: 45678,
      last_modified: now,
      homepage: 'https://example.com/comp',
      tags: ['basic','comprehensive']
    },
    {
      blocklist_id: 'bl-restrictive',
      name: 'Restrictive Ultra',
      description: 'Maximum blocking; may break sites.',
      entries: 78901,
      last_modified: now,
      homepage: 'https://example.com/restrictive',
      tags: ['basic','comprehensive','restrictive']
    },
    {
      blocklist_id: 'bl-hagezi',
      name: 'Hagezi Sources',
      description: 'Community curated tracker domains.',
      entries: 23456,
      last_modified: now,
      homepage: 'https://example.com/hagezi',
      tags: ['hagezi']
    }
  ];
}
