// Number formatting for the Statistics page.
//
// Source of truth: docs/specs/statistics-behaviour.md Section X (X4).
// If the rules change, update that spec and formatStats.test.ts with matching
// `tableRef: statistics-behaviour #N` annotations.

const COMPACT_FROM = 10_000;

export function formatCount(n: number): string {
    return n.toLocaleString();
}

/** Compact from 10,000 ("12.3K", "1.2M"); exact below. Pair with formatCount for the accessible name. */
export function formatCompact(n: number): string {
    if (Math.abs(n) < COMPACT_FROM) return formatCount(n);
    return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(n);
}

/** One decimal; "<0.1 %" for small non-zero shares; "-" when the denominator is 0. */
export function formatPercent(part: number, whole: number): string {
    if (!whole) return '-';
    const value = (part / whole) * 100;
    if (value > 0 && value < 0.05) return '<0.1 %';
    return `${value.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} %`;
}

/** Axis tick: whole thousands drop the decimal ("2K", "2.5K"). */
export function formatAxisCount(n: number): string {
    if (Math.abs(n) < 1000) return String(n);
    return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(n);
}
