'use client';

import { useEffect } from 'react';

/**
 * Enables the landing's entrance + scroll-reveal motion.
 *
 * Adds `is-ready` to <html> on mount, which is what arms the `.reveal` and
 * hero-stage animations in globals.css. Because those rules are scoped to
 * `html.is-ready`, the page renders fully visible without JS and in headless
 * renderers (crawlers, screenshotters) — the motion only ever enhances an
 * already-visible default, it never gates content on a class that might not
 * fire. Renders nothing.
 */
export function MotionReady() {
  useEffect(() => {
    const root = document.documentElement;
    root.classList.add('is-ready');

    const reveals = Array.from(document.querySelectorAll<HTMLElement>('.reveal'));
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    if (reduced || !('IntersectionObserver' in window)) {
      reveals.forEach((el) => el.setAttribute('data-inview', 'true'));
      return;
    }

    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.setAttribute('data-inview', 'true');
            io.unobserve(entry.target);
          }
        }
      },
      { rootMargin: '0px 0px -8% 0px', threshold: 0.08 },
    );
    reveals.forEach((el) => io.observe(el));

    return () => io.disconnect();
  }, []);

  return null;
}
