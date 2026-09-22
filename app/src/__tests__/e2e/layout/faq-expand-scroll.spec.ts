import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Expanding an FAQ answer while scrolled must keep the viewport where it is.
// Mobile projects only: the report is a phone-viewport regression.

const LAST_QUESTION = 'Do you support 2FA?';
// Height transition is 300ms; give layout time to settle before measuring.
const SETTLE_MS = 600;

async function openFaqScrolledToLastQuestion(page: Page) {
    await registerMocks(page);
    await page.goto('/faq');
    await page.waitForLoadState('networkidle');
    const lastQuestion = page.getByRole('button', { name: LAST_QUESTION });
    await lastQuestion.scrollIntoViewIfNeeded();
    await page.waitForTimeout(SETTLE_MS);
    const scrollYBefore = await page.evaluate(() => window.scrollY);
    expect(scrollYBefore).toBeGreaterThan(500);
    return { lastQuestion, scrollYBefore };
}

test.describe('FAQ expand keeps scroll position', { tag: '@mobile' }, () => {
    test('expanding the last answer does not move the viewport', async ({ page }) => {
        const { lastQuestion, scrollYBefore } = await openFaqScrolledToLastQuestion(page);
        await lastQuestion.click();
        await page.waitForTimeout(SETTLE_MS);
        const scrollYAfter = await page.evaluate(() => window.scrollY);
        expect(Math.abs(scrollYAfter - scrollYBefore)).toBeLessThan(8);
        await expect(page.getByText('Two-Factor Authentication adds an additional layer')).toBeInViewport();
    });

    test('expand all from a scrolled position does not jump to the top', async ({ page }) => {
        const { scrollYBefore } = await openFaqScrolledToLastQuestion(page);
        // The control sits at the top of the page; drive it without scrolling there.
        await page.getByRole('button', { name: 'Expand All' }).dispatchEvent('click');
        await page.waitForTimeout(SETTLE_MS);
        const scrollYAfter = await page.evaluate(() => window.scrollY);
        expect(scrollYAfter).toBeGreaterThan(0);
        // Content above grew, so the offset may rise; it must never fall back to the top.
        expect(scrollYAfter).toBeGreaterThanOrEqual(scrollYBefore - 8);
    });

    test('no scrollable space below the footer links', async ({ page }) => {
        await registerMocks(page);
        await page.goto('/faq');
        await page.waitForLoadState('networkidle');
        // Collapsed answers must not push the document past the footer: absolutely
        // positioned descendants need the clipped wrapper as their containing block.
        const { scrollHeight, footerBottom } = await page.evaluate(() => {
            const link = document.querySelector('a[title="Go to Terms of Service page"]') as HTMLElement;
            const footer = link.closest('div')!.parentElement as HTMLElement;
            return {
                scrollHeight: document.documentElement.scrollHeight,
                footerBottom: footer.getBoundingClientRect().bottom + window.scrollY,
            };
        });
        // 32px is the page wrapper's bottom padding.
        expect(scrollHeight - footerBottom).toBeLessThanOrEqual(40);
    });

    test('the document is the only vertical scroller', async ({ page }) => {
        await registerMocks(page);
        await page.goto('/faq');
        await page.waitForLoadState('networkidle');
        // A wrapper that clips or scrolls vertically AND holds more content than it
        // shows is a nested scroller; those are what confuse mobile engines.
        const nested = await page.evaluate(() => {
            const found: string[] = [];
            for (const el of document.querySelectorAll<HTMLElement>('body *')) {
                const overflowY = getComputedStyle(el).overflowY;
                if (overflowY === 'visible' || overflowY === 'clip') continue;
                // Collapsed answers (height 0) and sr-only spans clip on purpose.
                if (el.clientHeight <= 1) continue;
                if (el.scrollHeight > el.clientHeight + 1) {
                    found.push(`${el.tagName.toLowerCase()}.${el.className.toString().slice(0, 60)} ${el.clientHeight}/${el.scrollHeight}`);
                }
            }
            return found;
        });
        expect(nested).toEqual([]);
    });
});
