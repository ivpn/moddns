import { describe, expect, it } from 'vitest';
import { axisLabelBox } from '@/pages/statistics/axisHover';

describe('axisLabelBox', () => {
    const text = 'Oct 6, 14:00-15:00';

    it('centres the label on the hovered bucket', () => {
        // tableRef: statistics-behaviour #X9
        const b = axisLabelBox(300, text, 0, 600);
        expect(b.x + b.width / 2).toBe(300);
    });

    it('keeps the label inside the chart at both edges', () => {
        // tableRef: statistics-behaviour #X9
        expect(axisLabelBox(4, text, 0, 600).x).toBe(0);
        const right = axisLabelBox(598, text, 0, 600);
        expect(right.x + right.width).toBe(600);
    });

    it('widens with the text and never leaves the range when it does not fit', () => {
        // tableRef: statistics-behaviour #X9
        expect(axisLabelBox(50, 'Oct 5, 2026 (UTC day)', 0, 80).x).toBe(0);
        expect(axisLabelBox(100, 'Oct 5, 2026 (UTC day)', 0, 600).width).toBeGreaterThan(axisLabelBox(100, 'Oct 6', 0, 600).width);
    });
});
