import { useCallback, useEffect, useState } from 'react';

/**
 * Theme state machine.
 *
 * Three preferences, two appearances. The user picks one of:
 *   - 'system' → mirror the OS's prefers-color-scheme (default)
 *   - 'light'  → force light (locked)
 *   - 'dark'   → force dark (locked)
 *
 * The *appearance* is always derived: it is either the explicit lock,
 * or the result of the media query at this instant. We persist only the
 * preference so a user who picked "system" keeps following the OS even
 * after they close the tab and the OS later flips its scheme.
 *
 * Sync points (kept in lockstep by useTheme):
 *   1. <html data-theme="..."> drives the tokens.css token swap.
 *   2. The Radix Themes `appearance` prop receives the same value.
 *   3. documentElement.style.colorScheme tells the UA to repaint form
 *      widgets and scrollbars without a flash on theme flip. (The
 *      `<meta name="color-scheme">` declared in index.html is the
 *      *initial* value; the runtime swap goes through the style prop
 *      because Radix's appearance prop mutates the inline style and we
 *      want a single source of truth for the resolved appearance.)
 */

export type ThemePreference = 'system' | 'light' | 'dark';
export type ThemeAppearance = 'light' | 'dark';

const STORAGE_KEY = 'comax.theme-pref';
const DARK_MEDIA = '(prefers-color-scheme: dark)';

export function readThemePreference(): ThemePreference {
  if (typeof window === 'undefined') return 'system';
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored;
    }
  } catch {
    // localStorage can throw under private-browsing / disabled storage;
    // fall back to system rather than crashing the app.
  }
  return 'system';
}

export function resolveAppearance(pref: ThemePreference): ThemeAppearance {
  if (pref === 'light' || pref === 'dark') return pref;
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return 'dark';
  }
  return window.matchMedia(DARK_MEDIA).matches ? 'dark' : 'light';
}

function writeThemePreference(pref: ThemePreference): void {
  if (typeof window === 'undefined') return;
  // Defensive: keep the write side as strict as the read side. All
  // call sites are typed today (SegmentedControl emits ThemePreference),
  // but a future caller forwarding an unchecked string should not be
  // able to poison localStorage with a value readThemePreference would
  // then quietly throw away.
  if (pref !== 'system' && pref !== 'light' && pref !== 'dark') return;
  try {
    if (pref === 'system') {
      window.localStorage.removeItem(STORAGE_KEY);
    } else {
      window.localStorage.setItem(STORAGE_KEY, pref);
    }
  } catch {
    // Same rationale as readThemePreference: storage may be unavailable.
  }
}

export interface ThemeState {
  preference: ThemePreference;
  appearance: ThemeAppearance;
  setPreference: (next: ThemePreference) => void;
}

/**
 * React binding around the theme state. Subscribes to the OS media
 * query only when the preference is 'system' so explicit light/dark
 * users do not pay for an idle listener.
 */
export function useTheme(): ThemeState {
  const [preference, setPreferenceState] = useState<ThemePreference>(() => readThemePreference());
  const [appearance, setAppearance] = useState<ThemeAppearance>(() =>
    resolveAppearance(preference),
  );

  // Recompute appearance whenever the preference changes, and listen
  // for OS changes only while we're in 'system' mode.
  useEffect(() => {
    setAppearance(resolveAppearance(preference));
    if (preference !== 'system') return;
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const mq = window.matchMedia(DARK_MEDIA);
    const onChange = () => setAppearance(mq.matches ? 'dark' : 'light');
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [preference]);

  // Sync the DOM attributes that downstream styling reads from.
  useEffect(() => {
    if (typeof document === 'undefined') return;
    document.documentElement.setAttribute('data-theme', appearance);
    document.documentElement.style.colorScheme = appearance;
  }, [appearance]);

  const setPreference = useCallback((next: ThemePreference) => {
    writeThemePreference(next);
    setPreferenceState(next);
  }, []);

  return { preference, appearance, setPreference };
}
