import type { Config } from 'tailwindcss';

/*
 * Design tokens live in src/styles/tokens.css as CSS custom properties
 * so the same names work inside Radix Themes and ad-hoc CSS. Tailwind's
 * theme below mirrors the most-used tokens; anything not listed here
 * can still be reached via style={{ color: 'var(--color-text)' }}.
 *
 * Semantic colors (success/danger/warning) are deliberately exposed so
 * markup uses bg-success-soft / text-danger rather than raw Radix
 * --green-a3 — keeps light/dark parity per ECC web/design-quality.md.
 */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        surface: {
          DEFAULT: 'var(--color-surface)',
          elevated: 'var(--color-surface-elevated)',
          hover: 'var(--color-surface-hover)',
          active: 'var(--color-surface-active)',
        },
        panel: 'var(--color-panel)',
        text: {
          DEFAULT: 'var(--color-text)',
          subtle: 'var(--color-text-subtle)',
          faint: 'var(--color-text-faint)',
        },
        muted: 'var(--color-muted)',
        accent: {
          DEFAULT: 'var(--color-accent)',
          hover: 'var(--color-accent-hover)',
          active: 'var(--color-accent-active)',
          text: 'var(--color-accent-text)',
          soft: 'var(--color-accent-soft)',
          strong: 'var(--color-accent-strong)',
        },
        border: {
          DEFAULT: 'var(--color-border)',
          strong: 'var(--color-border-strong)',
        },
        success: {
          DEFAULT: 'var(--color-success)',
          soft: 'var(--color-success-soft)',
        },
        danger: {
          DEFAULT: 'var(--color-danger)',
          soft: 'var(--color-danger-soft)',
          strong: 'var(--color-danger-strong)',
        },
        warning: {
          DEFAULT: 'var(--color-warning)',
          soft: 'var(--color-warning-soft)',
        },
        info: {
          DEFAULT: 'var(--color-info)',
          soft: 'var(--color-info-soft)',
        },
        code: {
          DEFAULT: 'var(--color-code-bg)',
          text: 'var(--color-code-text)',
        },
        focus: 'var(--color-focus-ring)',
      },
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
      },
      fontSize: {
        xs: 'var(--text-xs)',
        sm: 'var(--text-sm)',
        base: 'var(--text-base)',
        md: 'var(--text-md)',
        lg: 'var(--text-lg)',
        xl: 'var(--text-xl)',
        '2xl': 'var(--text-2xl)',
        '3xl': 'var(--text-3xl)',
        display: 'var(--text-display)',
        hero: 'var(--text-hero)',
      },
      borderRadius: {
        xs: 'var(--radius-xs)',
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
      },
      boxShadow: {
        sm: 'var(--shadow-sm)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        drawer: 'var(--shadow-drawer)',
        palette: 'var(--shadow-palette)',
      },
      transitionDuration: {
        fast: 'var(--duration-fast)',
        normal: 'var(--duration-normal)',
        slow: 'var(--duration-slow)',
      },
      transitionTimingFunction: {
        'out-expo': 'var(--ease-out-expo)',
        'out-quart': 'var(--ease-out-quart)',
        'out-back': 'var(--ease-out-back)',
      },
    },
  },
  plugins: [],
} satisfies Config;
