import type { Config } from 'tailwindcss';

/*
 * Design tokens live in src/styles/tokens.css as CSS custom properties
 * so the same names work inside Radix Themes and ad-hoc CSS. Tailwind's
 * theme below mirrors the most-used tokens; anything not listed here
 * can still be reached via style={{ color: 'var(--color-text)' }}.
 *
 * Semantic colors (success/danger/warning) are deliberately exposed so
 * markup uses bg-success-soft / text-success rather than raw Radix
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
        },
        text: {
          DEFAULT: 'var(--color-text)',
          subtle: 'var(--color-text-subtle)',
        },
        muted: 'var(--color-muted)',
        accent: {
          DEFAULT: 'var(--color-accent)',
          text: 'var(--color-accent-text)',
          soft: 'var(--color-accent-soft)',
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
        },
        warning: {
          DEFAULT: 'var(--color-warning)',
          soft: 'var(--color-warning-soft)',
        },
        focus: 'var(--color-focus-ring)',
      },
      fontFamily: {
        sans: [
          'ui-sans-serif',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Inter',
          'sans-serif',
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      fontSize: {
        hero: 'var(--text-hero)',
        display: 'var(--text-display)',
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
      },
      transitionDuration: {
        fast: 'var(--duration-fast)',
        normal: 'var(--duration-normal)',
        slow: 'var(--duration-slow)',
      },
      transitionTimingFunction: {
        'out-expo': 'var(--ease-out-expo)',
        'out-back': 'var(--ease-out-back)',
      },
    },
  },
  plugins: [],
} satisfies Config;
