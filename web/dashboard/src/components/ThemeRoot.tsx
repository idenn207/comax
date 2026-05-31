import { createContext, useContext, type ReactNode } from 'react';
import { Theme } from '@radix-ui/themes';

import { useTheme, type ThemeState } from '../lib/theme';

/**
 * App-root theme wrapper. Owns the single useTheme() call and exposes
 * its state via context so deep children (e.g. ThemeToggle) can read
 * the preference without prop-drilling and without instantiating their
 * own state machine.
 *
 * Radix Themes' `appearance` prop is fed the *resolved* light/dark
 * value (not the user-facing 'system' preference) — Radix only knows
 * concrete appearances, so the system→OS mapping happens in useTheme().
 */

const ThemeStateContext = createContext<ThemeState | null>(null);

export function ThemeRoot({ children }: { children: ReactNode }) {
  const state = useTheme();
  return (
    <ThemeStateContext.Provider value={state}>
      <Theme appearance={state.appearance} accentColor="indigo" grayColor="slate" radius="medium">
        {children}
      </Theme>
    </ThemeStateContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useThemeContext(): ThemeState {
  const ctx = useContext(ThemeStateContext);
  if (!ctx) {
    // Production: a missing ThemeRoot is a wiring bug — a ThemeToggle
    // dropped into a tree without the Provider would silently pretend to
    // work, and the regression only surfaces when someone notices the
    // theme never changes. Fail loudly so it shows up in the first
    // render path. Vitest sets NODE_ENV='test' automatically, and Vite
    // statically substitutes process.env.NODE_ENV at build time, so the
    // production bundle has the `throw` inlined and the test build has
    // the fallback.
    if (process.env.NODE_ENV !== 'test') {
      throw new Error('useThemeContext must be used inside <ThemeRoot>');
    }
    // Test-only fallback: renderWithProviders does not wrap ThemeRoot,
    // so components that only incidentally call useThemeContext (or
    // never call it but live below a wrapper that does) should not need
    // an extra layer of boilerplate. The fallback is intentionally
    // no-op so a test that depends on theme behavior must mount the
    // real provider explicitly.
    return {
      preference: 'system',
      appearance: 'dark',
      setPreference: () => {
        /* no-op fallback */
      },
    };
  }
  return ctx;
}
