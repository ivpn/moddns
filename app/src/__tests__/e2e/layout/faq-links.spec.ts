import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Links in FAQ answers use the brand colour. The app's base stylesheet colours every
// <a>, so a link only gets the brand colour through an important utility class.
test.describe('FAQ links', { tag: '@desktop' }, () => {
    test('every link in an answer is rdns-600, including link text in a code box', async ({ page }) => {
        await registerMocks(page);
        await page.goto('/faq');
        await page.getByRole('button', { name: /expand all/i }).click();

        const rdns600 = await page.evaluate(() => {
            const probe = document.createElement('span');
            probe.style.color = 'var(--tailwind-colors-rdns-600)';
            document.body.appendChild(probe);
            const color = getComputedStyle(probe).color;
            probe.remove();
            return color;
        });
        const links = await page.getByTestId('faq-answer').locator('a').evaluateAll(els =>
            els.map(el => ({
                href: el.getAttribute('href'),
                color: getComputedStyle(el).color,
                textColor: getComputedStyle(el.firstElementChild ?? el).color,
            })),
        );
        expect(links.length).toBeGreaterThanOrEqual(7);
        for (const link of links) {
            expect(link.color, link.href ?? '').toBe(rdns600);
            expect(link.textColor, link.href ?? '').toBe(rdns600);
        }
    });
});
