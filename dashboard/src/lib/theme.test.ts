import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { readThemePreference, resolveAppearance, useTheme, type ThemePreference } from './theme';

/**
 * Theme hook contract:
 *   - default preference is 'system' when localStorage is empty / invalid
 *   - explicit pref persists across reads
 *   - 'system' pref resolves via window.matchMedia
 *   - setPreference('system') deletes the storage key
 *   - useTheme syncs <html data-theme> + color-scheme on the documentElement
 */

const STORAGE_KEY = 'comax.theme-pref';

function setMatchMediaResult(matches: boolean): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches,
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

describe('readThemePreference', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults to "system" when nothing is stored', () => {
    expect(readThemePreference()).toBe('system');
  });

  it('reads back a stored "light" preference', () => {
    localStorage.setItem(STORAGE_KEY, 'light');
    expect(readThemePreference()).toBe('light');
  });

  it('reads back a stored "dark" preference', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    expect(readThemePreference()).toBe('dark');
  });

  it('falls back to "system" when the stored value is garbage', () => {
    localStorage.setItem(STORAGE_KEY, 'plaid');
    expect(readThemePreference()).toBe('system');
  });
});

describe('resolveAppearance', () => {
  beforeEach(() => {
    setMatchMediaResult(false);
  });

  it('returns "light" when locked to light', () => {
    expect(resolveAppearance('light')).toBe('light');
  });

  it('returns "dark" when locked to dark', () => {
    expect(resolveAppearance('dark')).toBe('dark');
  });

  it('returns "light" when "system" and OS prefers light', () => {
    setMatchMediaResult(false);
    expect(resolveAppearance('system')).toBe('light');
  });

  it('returns "dark" when "system" and OS prefers dark', () => {
    setMatchMediaResult(true);
    expect(resolveAppearance('system')).toBe('dark');
  });
});

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear();
    setMatchMediaResult(false);
    document.documentElement.removeAttribute('data-theme');
    document.documentElement.style.colorScheme = '';
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('hydrates with system preference + media-derived appearance', () => {
    setMatchMediaResult(true);
    const { result } = renderHook(() => useTheme());
    expect(result.current.preference).toBe('system');
    expect(result.current.appearance).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });

  it('locks to light when setPreference("light") is called', () => {
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setPreference('light'));
    expect(result.current.preference).toBe('light');
    expect(result.current.appearance).toBe('light');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('locks to dark when setPreference("dark") is called', () => {
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setPreference('dark' satisfies ThemePreference));
    expect(result.current.preference).toBe('dark');
    expect(result.current.appearance).toBe('dark');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
  });

  it('removes the storage key when reverting to "system"', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setPreference('system'));
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it('toggles documentElement attributes when the appearance changes', () => {
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setPreference('light'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    act(() => result.current.setPreference('dark'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('ignores invalid preferences so localStorage cannot be poisoned', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    const { result } = renderHook(() => useTheme());
    // Call sites are typed, but a future regression handing us an
    // unchecked string should be dropped silently instead of persisted
    // (readThemePreference would discard it on the next read anyway).
    act(() => result.current.setPreference('plaid' as unknown as ThemePreference));
    expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
  });
});
