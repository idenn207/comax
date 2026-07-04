import type { Config } from 'tailwindcss';
import typography from '@tailwindcss/typography';

/*
 * Design tokens live in app/globals.css as CSS custom properties, mirrored
 * (name + value) from web/dashboard/src/styles/tokens.css so the marketing
 * site and the dashboard share one visual language. The `brand` family is the
 * ONE website-only addition (M6 D10): a single restrained chromatic accent for
 * CTA + hierarchy on the landing surface. All neutral/semantic tokens are kept
 * identical — check-token-parity.mjs enforces that (Codex F4).
 *
 * Rule: accent (brand) is used ≤1 per viewport (impeccable Output Constraint #2).
 */
export default {
  darkMode: ['selector', '[data-theme="dark"]'],
  content: [
    './app/**/*.{ts,tsx,mdx}',
    './components/**/*.{ts,tsx}',
    './content/**/*.{md,mdx}',
    './lib/**/*.{ts,tsx}',
  ],
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
        // brand — the single website-only chromatic accent (D10).
        brand: {
          DEFAULT: 'var(--color-brand)',
          hover: 'var(--color-brand-hover)',
          soft: 'var(--color-brand-soft)',
          text: 'var(--color-brand-text)',
        },
        border: {
          DEFAULT: 'var(--color-border)',
          strong: 'var(--color-border-strong)',
        },
        success: { DEFAULT: 'var(--color-success)', soft: 'var(--color-success-soft)' },
        danger: {
          DEFAULT: 'var(--color-danger)',
          soft: 'var(--color-danger-soft)',
          strong: 'var(--color-danger-strong)',
        },
        warning: { DEFAULT: 'var(--color-warning)', soft: 'var(--color-warning-soft)' },
        info: { DEFAULT: 'var(--color-info)', soft: 'var(--color-info-soft)' },
        code: { DEFAULT: 'var(--color-code-bg)', text: 'var(--color-code-text)' },
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
      },
      transitionDuration: {
        fast: 'var(--duration-fast)',
        normal: 'var(--duration-normal)',
        slow: 'var(--duration-slow)',
      },
      transitionTimingFunction: {
        'out-expo': 'var(--ease-out-expo)',
        'out-quart': 'var(--ease-out-quart)',
      },
      maxWidth: {
        content: 'var(--content-max)',
      },
    },
  },
  plugins: [typography],
} satisfies Config;
