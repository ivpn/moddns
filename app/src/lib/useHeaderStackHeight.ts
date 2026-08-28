import { useLayoutEffect } from 'react';
import type { RefObject } from 'react';

/**
 * Measures the combined height of fixed header elements and publishes it
 * to the CSS custom property --app-header-stack. Accepts an optional reducePx
 * to subtract a small gap (e.g. tighten space between header and content).
 */
export function useHeaderStackHeight(
  refs: Array<RefObject<HTMLElement | null>>,
  options: { reducePx?: number } = {}
) {
  const { reducePx = 0 } = options;

  useLayoutEffect(() => {
    const root = document.documentElement;
    let lastTotal = -1;

    function update() {
      const total = refs.reduce((acc, r) => acc + (r.current?.getBoundingClientRect().height || 0), 0);
      // Skip same-value writes: mobile URL-bar collapse/expand fires window resize
      // on every scroll direction change, and each setProperty invalidates root
      // style — repositioning var-consuming sticky elements a frame late (flicker).
      if (total === lastTotal) return;
      lastTotal = total;
      const adjusted = Math.max(0, total - reducePx);
      root.style.setProperty('--app-header-stack-full', total + 'px');
      root.style.setProperty('--app-header-stack', adjusted + 'px');
    }

    // Initial measure
    requestAnimationFrame(update);

    const observers = refs.map(r => {
      if (!r.current) return null;
      const ro = new ResizeObserver(update);
      ro.observe(r.current);
      return ro;
    });

    window.addEventListener('resize', update);

    return () => {
      window.removeEventListener('resize', update);
      observers.forEach(o => o?.disconnect());
    };
  }, [refs, reducePx]);
}
