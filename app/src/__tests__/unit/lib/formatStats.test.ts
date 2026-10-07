import { describe, it, expect } from 'vitest';
import { formatAxisCount, formatCompact, formatCount, formatPercent } from '@/lib/formatStats';

describe('formatStats', () => {
    it('formats counts with locale separators', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatCount(1234567)).toBe((1234567).toLocaleString());
        expect(formatCount(0)).toBe('0');
    });

    it('keeps exact values below 10,000', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatCompact(9999)).toBe((9999).toLocaleString());
        expect(formatCompact(0)).toBe('0');
    });

    it('compacts from 10,000', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatCompact(12345)).toBe('12.3K');
        expect(formatCompact(10000)).toBe('10K');
        expect(formatCompact(1_234_567)).toBe('1.2M');
    });

    it('renders percent with one decimal', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatPercent(1, 8)).toBe('12.5 %');
        expect(formatPercent(0, 100)).toBe('0.0 %');
        expect(formatPercent(100, 100)).toBe('100.0 %');
    });

    it('renders "<0.1 %" for small non-zero shares', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatPercent(1, 10000)).toBe('<0.1 %');
    });

    it('renders an em dash when the denominator is 0', () => {
        // tableRef: statistics-behaviour #X4
        expect(formatPercent(0, 0)).toBe('—');
        expect(formatPercent(5, 0)).toBe('—');
    });

    it('formats axis ticks', () => {
        expect(formatAxisCount(500)).toBe('500');
        expect(formatAxisCount(2000)).toBe('2K');
        expect(formatAxisCount(2500)).toBe('2.5K');
    });
});
