import { useEffect, useState } from 'react';

/**
 * True once the window has scrolled past `threshold` px. Used by the app
 * chrome to materialize the header's edge (hairline + shadow) only while
 * content is actually scrolled underneath it.
 */
export function useScrolled(threshold = 4): boolean {
    const [scrolled, setScrolled] = useState(false);

    useEffect(() => {
        const onScroll = () => {
            const shouldBeScrolled = window.scrollY > threshold;
            setScrolled((prev) => prev === shouldBeScrolled ? prev : shouldBeScrolled);
        };
        window.addEventListener('scroll', onScroll, { passive: true });
        onScroll();
        return () => window.removeEventListener('scroll', onScroll);
    }, [threshold]);

    return scrolled;
}
