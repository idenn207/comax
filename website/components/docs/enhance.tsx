'use client';

import { useEffect } from 'react';
import { usePathname } from 'next/navigation';

// Inline SVGs (no icon-font, no network). Copy → check on success.
const COPY_SVG =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a1 1 0 0 1 1-1h9" stroke-linecap="round"/></svg>';
const CHECK_SVG =
  '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M5 12.5 10 17.5 19 7" stroke-linecap="round" stroke-linejoin="round"/></svg>';

/**
 * Progressive enhancement for docs code cards. Gives every rehype-pretty-code
 * figure a labelled bar (matching the claude_design "terminal" card) and a copy
 * button. The code itself is server-rendered and fully visible without JS; this
 * only adds the bar + copy on hydration. Re-runs on client navigation (pathname
 * dep) so figures on the next doc get enhanced too.
 */
export function DocsEnhance() {
  const pathname = usePathname();

  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    const figures = document.querySelectorAll<HTMLElement>('[data-rehype-pretty-code-figure]');

    figures.forEach((figure) => {
      if (figure.querySelector('.code-copy')) return; // already enhanced
      const pre = figure.querySelector('pre');
      if (!pre) return;

      // Ensure a bar exists (fences without a `title=` get a default one).
      let bar = figure.querySelector<HTMLElement>(
        '[data-rehype-pretty-code-title], .code-card-bar',
      );
      if (!bar) {
        bar = document.createElement('div');
        bar.className = 'code-card-bar';
        const label = document.createElement('span');
        label.textContent = 'terminal';
        bar.appendChild(label);
        figure.insertBefore(bar, pre);
      }

      const setIdle = (btn: HTMLButtonElement) => {
        btn.dataset.copied = 'false';
        btn.innerHTML = `${COPY_SVG}<span>복사</span>`;
      };

      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'code-copy';
      btn.setAttribute('aria-label', '코드 복사');
      setIdle(btn);

      btn.addEventListener('click', () => {
        // textContent alone drops newlines (grid rows are blocks), so join the
        // per-line nodes rehype-pretty-code emits.
        const lineNodes = pre.querySelectorAll('[data-line]');
        const code = (
          lineNodes.length
            ? Array.from(lineNodes, (l) => l.textContent ?? '').join('\n')
            : (pre.textContent ?? '')
        ).replace(/\r/g, ''); // strip CR so pasted commands don't carry CRLF tails
        void navigator.clipboard?.writeText(code).then(() => {
          btn.dataset.copied = 'true';
          btn.innerHTML = `${CHECK_SVG}<span>복사됨</span>`;
          timers.push(setTimeout(() => setIdle(btn), 1600));
        });
      });

      bar.appendChild(btn);
    });

    return () => timers.forEach(clearTimeout);
  }, [pathname]);

  return null;
}
