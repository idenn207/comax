import type { Config } from 'tailwindcss';

// Design tokens live in src/styles/tokens.css as CSS custom properties
// so the same names work inside Radix Themes and ad-hoc CSS. Tailwind's
// theme below mirrors the most-used tokens; anything not listed here
// can still be reached via `style={{ color: 'var(--color-text)' }}`.
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        surface: 'var(--color-surface)',
        text: 'var(--color-text)',
        muted: 'var(--color-muted)',
        accent: 'var(--color-accent)',
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
      transitionTimingFunction: {
        'out-expo': 'var(--ease-out-expo)',
      },
    },
  },
  plugins: [],
} satisfies Config;
