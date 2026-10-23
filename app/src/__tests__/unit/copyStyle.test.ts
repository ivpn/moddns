import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';

const root = resolve(__dirname, '../..');

function sources(dir: string): string[] {
    return readdirSync(dir).flatMap(name => {
        const p = join(dir, name);
        return statSync(p).isDirectory() ? sources(p) : /\.(tsx?|css)$/.test(name) ? [p] : [];
    });
}

describe('copy style', () => {
    it('uses short hyphens only in the Statistics page, the data collection control and the FAQ', () => {
        const files = [...sources(join(root, 'pages/statistics')), ...sources(join(root, 'components/data-collection')), join(root, 'pages/legal/FAQ.tsx')];
        const offenders = files.filter(f => /[–—]/.test(readFileSync(f, 'utf8'))).map(f => f.slice(root.length + 1));
        expect(offenders).toEqual([]);
    });
});
