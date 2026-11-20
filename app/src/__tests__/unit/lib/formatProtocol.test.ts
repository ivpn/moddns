import { describe, it, expect } from 'vitest';
import { formatProtocol, formatProtocolDetail } from '@/lib/formatProtocol';

describe('formatProtocol', () => {
    it('upper-cases the logged transport and shows DoH over HTTP/3 as HTTPS', () => {
        // tableRef: proxy-request-admission-behaviour #Q18
        expect(formatProtocol('https')).toBe('HTTPS');
        expect(formatProtocol('doh3')).toBe('HTTPS');
        expect(formatProtocol('tls')).toBe('TLS');
        expect(formatProtocol('quic')).toBe('QUIC');
        expect(formatProtocol('udp')).toBe('UDP');
        expect(formatProtocol('tcp')).toBe('TCP');
    });

    it('shows a dash when there is no protocol', () => {
        // tableRef: proxy-request-admission-behaviour #Q18
        expect(formatProtocol(undefined)).toBe('-');
        expect(formatProtocol('')).toBe('-');
    });
});

describe('formatProtocolDetail', () => {
    it('adds HTTP/3 only for doh3', () => {
        // tableRef: proxy-request-admission-behaviour #Q18
        expect(formatProtocolDetail('doh3')).toBe('HTTPS (HTTP/3)');
        expect(formatProtocolDetail('https')).toBe('HTTPS');
        expect(formatProtocolDetail('quic')).toBe('QUIC');
        expect(formatProtocolDetail(undefined)).toBe('-');
    });
});
