// @vitest-environment jsdom
import { beforeAll, describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { normalizeStats } from '@/pages/statistics/derive';
import { KpiCards } from '@/pages/statistics/panels/KpiCards';
import { SeriesPanel, chartSummary } from '@/pages/statistics/panels/SeriesPanel';
import { ReasonsPanel } from '@/pages/statistics/panels/ReasonsPanel';
import { ProtocolsPanel } from '@/pages/statistics/panels/ProtocolsPanel';
import { DevicesPanel } from '@/pages/statistics/panels/DevicesPanel';
import { DomainsPanel } from '@/pages/statistics/panels/DomainsPanel';
import { ClientsPanel } from '@/pages/statistics/panels/ClientsPanel';
import { buildBuckets } from '@/pages/statistics/derive';
import { createStatsResponse, topBlocked, topClients } from '../mocks/statisticsMocks';

beforeAll(() => {
    // Recharts' ResponsiveContainer observes its parent.
    globalThis.ResizeObserver ??= class {
        observe() {}
        unobserve() {}
        disconnect() {}
    } as unknown as typeof ResizeObserver;
});

const data = normalizeStats(createStatsResponse({ points: 48 }));

describe('KpiCards', () => {
    it('shows exact values in the accessible name and one-decimal percent', () => {
        // tableRef: statistics-behaviour #X4
        const big = normalizeStats({ ...createStatsResponse(), totals: { total: 12345, blocked: 1234, dnssec: 8901 } });
        render(<KpiCards data={big} />);
        const total = screen.getByLabelText('Total queries: 12,345');
        expect(total).toHaveTextContent('12.3K');
        expect(total).toHaveAttribute('title', '12,345');
        expect(screen.getByLabelText('Blocked: 1,234')).toHaveTextContent('1,234');
        expect(screen.getByLabelText('Blocked %: 10.0 %')).toBeInTheDocument();
        expect(screen.getByText('1,234 of 12,345')).toBeInTheDocument();
        expect(screen.getByText('72.1 % of total')).toBeInTheDocument();
    });

    it('shows 0 and 0.0 % when nothing was blocked', () => {
        // tableRef: statistics-behaviour #P12
        const none = normalizeStats(createStatsResponse({ blocked: false }));
        render(<KpiCards data={none} />);
        expect(screen.getByLabelText('Blocked: 0')).toHaveTextContent('0');
        expect(screen.getByLabelText('Blocked %: 0.0 %')).toBeInTheDocument();
    });

    it('hides sparklines from assistive technology', () => {
        // tableRef: statistics-behaviour #X2
        const { container } = render(<KpiCards data={data} />);
        const svgs = container.querySelectorAll('svg');
        expect(svgs.length).toBeGreaterThan(0);
        svgs.forEach(s => expect(s).toHaveAttribute('aria-hidden', 'true'));
    });
});

describe('SeriesPanel', () => {
    it('labels the chart with a computed summary', () => {
        // tableRef: statistics-behaviour #X2
        render(<SeriesPanel data={data} range="7d" />);
        const group = screen.getByRole('group', { name: /^Queries over the last 7 days: .* total, .* blocked; busiest/ });
        expect(group).toHaveAttribute('aria-roledescription', 'chart');
        expect(chartSummary(data, buildBuckets(data), '7d')).toContain(data.totals.total.toLocaleString());
    });

    it('offers a real table with caption and column headers, newest first', async () => {
        // tableRef: statistics-behaviour #X3
        const user = userEvent.setup();
        render(<SeriesPanel data={data} range="7d" />);
        await user.click(screen.getByRole('button', { name: 'Table' }));
        const table = screen.getByRole('table');
        expect(within(table).getByText('Hourly buckets, newest first')).toBeInTheDocument();
        expect(within(table).getAllByRole('columnheader').map(h => h.textContent)).toEqual(['Time', 'Total', 'Blocked', 'DNSSEC']);
        const rows = within(table).getAllByRole('row');
        expect(rows.length).toBe(49);
        expect(rows[1]).toHaveTextContent('(in progress)');
    });

    it('omits the gap before statistics were enabled from the table and says so', async () => {
        // tableRef: statistics-behaviour #P9
        const user = userEvent.setup();
        const gap = normalizeStats(createStatsResponse({ points: 8, enabledAt: '2026-10-06T13:20:00Z' }));
        render(<SeriesPanel data={gap} range="24h" />);
        expect(screen.getByText(/Counting since .*Earlier hours are blank, not zero\./)).toBeInTheDocument();
        await user.click(screen.getByRole('button', { name: 'Table' }));
        expect(within(screen.getByRole('table')).getAllByRole('row').length).toBeLessThan(9);
    });

    it('reduces opacity while a new range loads', () => {
        // tableRef: statistics-behaviour #P14
        const { container } = render(<SeriesPanel data={data} range="7d" busy />);
        expect(container.querySelector('.opacity-60')).not.toBeNull();
    });
});

describe('ReasonsPanel', () => {
    it('renders a sorted bar list with text on every row', () => {
        // tableRef: statistics-behaviour #X6, #X5
        render(<ReasonsPanel data={data} range="7d" />);
        const items = within(screen.getByRole('region', { name: 'Blocked by reason' }) ?? document.body).getAllByRole('listitem');
        expect(items[0]).toHaveTextContent('Blocklists');
        expect(items.map(i => i.textContent).join(' ')).toMatch(/Services.*Custom rules.*Default rule/);
    });

    it('has a table view', async () => {
        // tableRef: statistics-behaviour #X3
        const user = userEvent.setup();
        render(<ReasonsPanel data={data} range="7d" />);
        await user.click(screen.getByRole('button', { name: 'Table' }));
        expect(screen.getByRole('table')).toBeInTheDocument();
        expect(screen.getByText(/Blocked queries by reason, last 7 days/)).toBeInTheDocument();
    });

    it('says nothing was blocked instead of drawing an empty chart', () => {
        // tableRef: statistics-behaviour #P12, #P21
        render(<ReasonsPanel data={normalizeStats(createStatsResponse({ blocked: false }))} range="7d" />);
        expect(screen.getByText('Nothing was blocked in this range.')).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Table' })).not.toBeInTheDocument();
    });
});

describe('ProtocolsPanel', () => {
    it('renders a donut with a text legend and percentages', () => {
        // tableRef: statistics-behaviour #X6
        render(<ProtocolsPanel data={data} range="7d" />);
        expect(screen.getByRole('img', { name: /^Protocols: DoH .* %, DoT .* %, DoQ .* %$/ })).toBeInTheDocument();
        expect(screen.getAllByText(/DoH|DoT|DoQ/).length).toBeGreaterThanOrEqual(3);
    });

    it('does not draw an empty donut', () => {
        // tableRef: statistics-behaviour #P21
        const none = normalizeStats(createStatsResponse({ empty: true }));
        render(<ProtocolsPanel data={none} range="7d" />);
        expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });
});

describe('DevicesPanel', () => {
    const lastSeen = new Map([['laptop', Date.now() - 120_000]]);

    it('labels the empty id and the grouped remainder', () => {
        // tableRef: statistics-behaviour #P20
        render(<DevicesPanel data={data} range="7d" lastSeen={null} />);
        expect(screen.getByText('laptop')).toHaveClass('font-mono');
        expect(screen.getByText('No device ID')).toBeInTheDocument();
        expect(screen.getByText('Other devices')).toBeInTheDocument();
    });

    it('omits last seen when logs are off', async () => {
        // tableRef: statistics-behaviour #P2
        const user = userEvent.setup();
        render(<DevicesPanel data={data} range="7d" lastSeen={null} />);
        expect(screen.queryByText(/last seen in logs/)).not.toBeInTheDocument();
        await user.click(screen.getByRole('button', { name: 'Table' }));
        expect(screen.queryByRole('columnheader', { name: 'Last seen in logs' })).not.toBeInTheDocument();
    });

    it('adds last seen in logs when logs are on', () => {
        // tableRef: statistics-behaviour #P3
        render(<DevicesPanel data={data} range="7d" lastSeen={lastSeen} />);
        expect(screen.getByText(/last seen in logs/)).toBeInTheDocument();
    });

    it('collapses long lists to five rows with an expander', async () => {
        const user = userEvent.setup();
        const many = normalizeStats({
            ...createStatsResponse(),
            devices: Array.from({ length: 8 }, (_, i) => ({ device_id: `d${i}`, total: 100 - i, blocked: 1 })),
        });
        render(<DevicesPanel data={many} range="7d" lastSeen={null} />);
        expect(screen.getAllByRole('listitem')).toHaveLength(5);
        await user.click(screen.getByRole('button', { name: 'Show 8 devices' }));
        expect(screen.getAllByRole('listitem')).toHaveLength(8);
    });
});

describe('DomainsPanel', () => {
    it('lists domains with counts and a table view', async () => {
        // tableRef: statistics-behaviour #X3
        const user = userEvent.setup();
        render(<DomainsPanel kind="blocked" items={topBlocked.items} range="7d" windowWords="1 hour" />);
        expect(screen.getByText('ads.example.com')).toBeInTheDocument();
        await user.click(screen.getByRole('button', { name: 'Table' }));
        expect(screen.getByRole('table')).toBeInTheDocument();
        expect(screen.getByText(/Top blocked domains, 1 hour of query logs/)).toBeInTheDocument();
    });

    it('says so instead of drawing an empty list', () => {
        // tableRef: statistics-behaviour #P21
        render(<DomainsPanel kind="blocked" items={[]} range="7d" windowWords="1 hour" />);
        expect(screen.getByText('No blocked domains in this range.')).toBeInTheDocument();
    });
});

describe('ClientsPanel', () => {
    it('renders IP, queries, ISP and country', () => {
        render(<ClientsPanel items={[{ ip: '203.0.113.7', count: 300, asn: 64500, asOrg: 'Example ISP', country: 'PL' }]} windowWords="1 day" />);
        const table = screen.getAllByRole('table', { hidden: true })[0];
        expect(within(table).getByText('Example ISP (AS64500)')).toBeInTheDocument();
        expect(within(table).getByText('PL')).toBeInTheDocument();
        expect(screen.queryByText('ISP and country unavailable')).not.toBeInTheDocument();
    });

    it('shows hyphens and a footnote when GeoIP data is missing', () => {
        // tableRef: statistics-behaviour #P19
        render(<ClientsPanel items={[{ ip: '198.51.100.2', count: 5, asn: null, asOrg: null, country: null }]} windowWords="1 day" />);
        expect(screen.getByText('ISP and country unavailable')).toBeInTheDocument();
        expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2);
        expect(topClients.items.length).toBe(2);
    });
});
