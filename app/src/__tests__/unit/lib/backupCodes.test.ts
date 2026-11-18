import { describe, it, expect } from 'vitest';
import { formatBackupCode } from '@/lib/backupCodes';

describe('formatBackupCode', () => {
    it('groups a 25-character code in fives', () => {
        // specRef: api-endpoint-behaviour #E5
        expect(formatBackupCode('GFUKSZ7KZBVPNDNTMWMSHAKR5')).toBe('GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5');
    });

    it('leaves a trailing short group intact', () => {
        // specRef: api-endpoint-behaviour #E5
        expect(formatBackupCode('AbCdEfG')).toBe('AbCdE-fG');
        expect(formatBackupCode('AbCdE')).toBe('AbCdE');
        expect(formatBackupCode('')).toBe('');
    });
});
