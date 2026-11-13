import { describe, expect, it } from 'vitest';
import { hasUnverifiedPasskey, passkeyVerificationDeadline, PASSKEY_VERIFICATION_REQUIRED_FROM } from '@/lib/passkeyVerification';

// specRef: api-endpoint-behaviour F16
describe('passkey verification deadline', () => {
  it('formats the announced date in UTC', () => {
    expect(PASSKEY_VERIFICATION_REQUIRED_FROM).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(passkeyVerificationDeadline('2026-12-31')).toBe('December 31, 2026');
    expect(passkeyVerificationDeadline()).toBe('December 31, 2026');
    expect(passkeyVerificationDeadline('2027-12-31')).toBe('December 31, 2027');
  });

  it('flags only passkeys whose latest sign-in reported no user verification', () => {
    expect(hasUnverifiedPasskey([{ id: 'a', user_verified: true }, { id: 'b', user_verified: false }])).toBe(true);
    expect(hasUnverifiedPasskey([{ id: 'a', user_verified: true }])).toBe(false);
    expect(hasUnverifiedPasskey([{ id: 'a' }])).toBe(false);
    expect(hasUnverifiedPasskey([])).toBe(false);
    expect(hasUnverifiedPasskey(undefined)).toBe(false);
  });
});
