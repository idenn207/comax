import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// jsdom doesn't ship ResizeObserver. Radix's scroll-area (which
// Select/Dialog mount internally) reaches for it on layout effect, so
// any test that opens a Select or scrollable popover crashes with a
// ReferenceError before our assertion runs. A no-op stub is enough —
// the dashboard doesn't observe size changes in unit tests.
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

// Same reason: Radix Select's positioner uses `scrollIntoView` on the
// active item; jsdom returns undefined and the type chain falls over
// during animation cleanup. The polyfill is a no-op.
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {};
}

// PointerEvent is jsdom-shimmed but the constructor doesn't accept the
// modern `pointerType` field; Radix's pointer-down handlers crash without
// it. Patch the prototype so userEvent's click drives Select correctly.
if (typeof globalThis.PointerEvent === 'undefined') {
  globalThis.PointerEvent = class extends MouseEvent {
    pointerType: string;
    constructor(type: string, props: PointerEventInit = {}) {
      super(type, props);
      this.pointerType = props.pointerType ?? 'mouse';
    }
  } as unknown as typeof PointerEvent;
}

// jsdom doesn't ship matchMedia. The theme hook reads
// `(prefers-color-scheme: dark)` to decide the system appearance, and
// any component tree that mounts ThemeRoot needs a working stub. We
// default to "not dark" so tests start in a deterministic light
// appearance; individual tests can override via vi.spyOn if they need
// to assert system→OS coupling.
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

// jsdom keeps state across tests inside the same file unless we tell it
// to wipe between cases. Without this, sessionStorage from test 1
// leaks into test 2 and the auth tests fail in mysterious ways.
afterEach(() => {
  cleanup();
  sessionStorage.clear();
  localStorage.clear();
});
