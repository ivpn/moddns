/** Moves focus to the visible page title in the header (desktop and mobile each render one). */
export function focusPageHeading(): void {
    requestAnimationFrame(() => {
        const headings = Array.from(document.querySelectorAll<HTMLElement>("[data-page-heading]"));
        const visible = headings.find(h => h.offsetParent !== null) ?? headings[0];
        visible?.focus({ preventScroll: true });
    });
}
