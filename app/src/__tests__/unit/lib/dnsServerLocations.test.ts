import { describe, it, expect } from 'vitest';
import { parseDnsServerLocations, firstAddress } from '@/lib/dnsServerLocations';

const DOMAIN = 'dns.example.net';

describe('parseDnsServerLocations', () => {
    it('parses prefix|City|ipv4|ipv6 entries', () => {
        const raw = 'ams1|Amsterdam|185.229.190.69|2a02:6ea0:100a::1,lax1|Los Angeles|107.155.127.138|2604:4500:8:2b::2';
        expect(parseDnsServerLocations(raw, DOMAIN)).toEqual([
            { prefix: 'ams1', city: 'Amsterdam', hostname: 'ams1.dns.example.net', ipv4: '185.229.190.69', ipv6: '2a02:6ea0:100a::1' },
            { prefix: 'lax1', city: 'Los Angeles', hostname: 'lax1.dns.example.net', ipv4: '107.155.127.138', ipv6: '2604:4500:8:2b::2' },
        ]);
    });

    it('still accepts the legacy prefix:City format without addresses', () => {
        expect(parseDnsServerLocations('ams1:Amsterdam,syd1:Sydney', DOMAIN)).toEqual([
            { prefix: 'ams1', city: 'Amsterdam', hostname: 'ams1.dns.example.net', ipv4: undefined, ipv6: undefined },
            { prefix: 'syd1', city: 'Sydney', hostname: 'syd1.dns.example.net', ipv4: undefined, ipv6: undefined },
        ]);
    });

    it('falls back to the prefix as city and leaves blank addresses undefined', () => {
        expect(parseDnsServerLocations('vm1|||', DOMAIN)).toEqual([
            { prefix: 'vm1', city: 'vm1', hostname: 'vm1.dns.example.net', ipv4: undefined, ipv6: undefined },
        ]);
        expect(parseDnsServerLocations('vm1|Quebec|51.161.64.178', DOMAIN)[0]).toMatchObject({
            city: 'Quebec', ipv4: '51.161.64.178', ipv6: undefined,
        });
        expect(parseDnsServerLocations('vm1', DOMAIN)[0]).toMatchObject({ prefix: 'vm1', city: 'vm1' });
    });

    it('trims whitespace and skips empty entries', () => {
        expect(parseDnsServerLocations(' ams1 | Amsterdam | 1.2.3.4 | 2001:db8::1 , , tor1:Toronto ,', DOMAIN)).toEqual([
            { prefix: 'ams1', city: 'Amsterdam', hostname: 'ams1.dns.example.net', ipv4: '1.2.3.4', ipv6: '2001:db8::1' },
            { prefix: 'tor1', city: 'Toronto', hostname: 'tor1.dns.example.net', ipv4: undefined, ipv6: undefined },
        ]);
    });

    it('returns an empty list for empty or undefined input', () => {
        expect(parseDnsServerLocations('', DOMAIN)).toEqual([]);
        expect(parseDnsServerLocations(undefined, DOMAIN)).toEqual([]);
    });
});

describe('firstAddress', () => {
    it('returns the first non-empty comma-separated item', () => {
        expect(firstAddress('89.124.253.5')).toBe('89.124.253.5');
        expect(firstAddress(' , 51.161.64.178 ,51.161.64.179')).toBe('51.161.64.178');
    });

    it('returns undefined when nothing is configured', () => {
        expect(firstAddress('')).toBeUndefined();
        expect(firstAddress(undefined)).toBeUndefined();
        expect(firstAddress(' , ')).toBeUndefined();
    });
});
