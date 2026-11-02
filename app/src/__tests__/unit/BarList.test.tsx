// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BarList } from '@/pages/statistics/primitives';
import { DevicesPanel } from '@/pages/statistics/panels/DevicesPanel';
import { createStatsResponse } from '../mocks/statisticsMocks';
import { normalizeStats } from '@/pages/statistics/derive';

const rows = (values: number[]) => values.map((v, i) => ({ id: `r${i}`, label: `row ${i}`, value: v, color: 'red', meta: String(v) }));
const widths = () => screen.getAllByTestId('bar-row').map(r => Array.from(r.querySelectorAll<HTMLElement>('[data-testid="bar-seg"]')).map(s => s.style.width));

describe('BarList scale', () => {
    it('share: each bar is its part of the total on a track', () => {
        // tableRef: statistics-behaviour #X7
        render(<BarList scale="share" rows={rows([900, 334])} />);
        const [first, second] = widths();
        expect(parseFloat(first[0])).toBeCloseTo(72.93, 1);
        expect(parseFloat(second[0])).toBeCloseTo(27.07, 1);
        for (const r of screen.getAllByTestId('bar-row')) {
            expect(r.className).toContain('--stats-track');
            expect(r).toHaveAttribute('aria-hidden', 'true');
        }
    });

    it('share: the denominator includes rows hidden behind "Show all"', () => {
        // tableRef: statistics-behaviour #X7
        render(<BarList scale="share" collapseAfter={1} rows={rows([500, 500])} />);
        expect(widths()).toEqual([['50%']]);
    });

    it('leader: the first bar is full width relative to the largest row, on the track', () => {
        // tableRef: statistics-behaviour #X7
        render(<BarList scale="leader" rows={rows([400, 100])} />);
        expect(widths()).toEqual([['100%'], ['25%']]);
        for (const r of screen.getAllByTestId('bar-row')) {
            expect(r.className).toContain('--stats-track');
        }
    });

    it('gives a tiny non-zero row a visible minimum and a zero row no bar', () => {
        // tableRef: statistics-behaviour #X7
        render(<BarList scale="share" rows={rows([100000, 1, 0])} />);
        const segs = screen.getAllByTestId('bar-seg');
        expect(segs).toHaveLength(2);
        expect(segs[1].style.minWidth).toBe('2px');
        expect(widths()[2]).toEqual([]);
    });

    it('splits a blocked segment inside the same denominator', () => {
        // tableRef: statistics-behaviour #X7
        render(<BarList scale="share" rows={[{ id: 'a', label: 'a', value: 600, blocked: 200, meta: '' }, { id: 'b', label: 'b', value: 400, meta: '', color: 'x' }]} />);
        expect(widths()[0]).toEqual(['40%', '20%']);
    });
});

describe('Devices bar text', () => {
    it('shows the share of all devices next to the count', () => {
        // tableRef: statistics-behaviour #X7
        const data = normalizeStats(createStatsResponse({ points: 48 }));
        const total = data.devices.reduce((s, d) => s + d.total, 0);
        render(<DevicesPanel data={data} range="7d" lastSeen={null} />);
        const first = data.devices[0];
        const share = ((first.total / total) * 100).toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
        expect(screen.getAllByText(new RegExp(`${share} %`))[0]).toBeInTheDocument();
    });
});
