import { useEffect, useState } from 'react';

// Simplified screen detector returning the original API shape:
// { width, isMobile, isTablet, isDesktop }
// Now: isDesktop is capability-aware (hover + fine pointer + min-width >=1024) instead of pure width.
export const useScreenDetector = () => {
  // navDesktop semantics:
  // - isDesktop: capability desktop (>=1024px + hover + fine pointer) used for density & certain desktop-only widgets.
  // - navDesktop: stricter; requires capability desktop, non-coarse pointer, and width >=1280.
  //   This prevents large landscape tablets (1024–1194) from receiving persistent sidebar/navigation; they get mobile overlay instead.
  const getInitialWidth = () => (typeof window !== 'undefined' ? window.innerWidth : 1920);
  const [width, setWidth] = useState<number>(getInitialWidth);
  const DESKTOP_MQ = '(min-width:1024px) and (hover:hover) and (pointer:fine)';
  const getInitialDesktop = () => (typeof window !== 'undefined' && window.matchMedia ? window.matchMedia(DESKTOP_MQ).matches : false);
  const [isDesktop, setIsDesktop] = useState<boolean>(getInitialDesktop);
  // Internal capability for differentiating large touch tablets from full desktop nav
  const COARSE_MQ = '(pointer:coarse)';
  const [isCoarsePointer, setIsCoarsePointer] = useState<boolean>(() => (typeof window !== 'undefined' && window.matchMedia ? window.matchMedia(COARSE_MQ).matches : false));

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const handleResize = () => setWidth(window.innerWidth);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return;
    const desktopMq = window.matchMedia(DESKTOP_MQ);
    const coarseMq = window.matchMedia(COARSE_MQ);
  const orientMq = window.matchMedia('(orientation: portrait)'); // kept for potential future use

    const handleDesktop = (e?: MediaQueryListEvent) => setIsDesktop(e ? e.matches : desktopMq.matches);
    const handleCoarse = (e?: MediaQueryListEvent) => setIsCoarsePointer(e ? e.matches : coarseMq.matches);
  const handleOrient = () => {}; // no-op until orientation needed

    // Initial sync
    handleDesktop();
    handleCoarse();
  handleOrient();

    const add = (mq: MediaQueryList, fn: (e: MediaQueryListEvent) => void) => {
      if (mq.addEventListener) mq.addEventListener('change', fn); else if ((mq as unknown as { addListener?: (fn: (e: MediaQueryListEvent) => void) => void }).addListener) (mq as unknown as { addListener: (fn: (e: MediaQueryListEvent) => void) => void }).addListener(fn);
    };
    const remove = (mq: MediaQueryList, fn: (e: MediaQueryListEvent) => void) => {
      if (mq.removeEventListener) mq.removeEventListener('change', fn); else if ((mq as unknown as { removeListener?: (fn: (e: MediaQueryListEvent) => void) => void }).removeListener) (mq as unknown as { removeListener: (fn: (e: MediaQueryListEvent) => void) => void }).removeListener(fn);
    };

    add(desktopMq, handleDesktop);
    add(coarseMq, handleCoarse);
  add(orientMq, handleOrient); // harmless listener now
    return () => {
      remove(desktopMq, handleDesktop);
      remove(coarseMq, handleCoarse);
  remove(orientMq, handleOrient);
    };
  }, []);

  const isMobile = width <= 768;
  const isTablet = width <= 1024; // width-based, kept for compatibility (tablet portrait & some landscape).

  // navDesktop: restrict full desktop navigation to wide (>=1280) capability-desktop & not coarse pointer (excludes landscape tablets 1024-1194)
  const navDesktop = isDesktop && !isCoarsePointer && width >= 1280;

  return { width, isMobile, isTablet, isDesktop, navDesktop };
};
