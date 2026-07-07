import type { CSSProperties } from 'react';

/**
 * Hero graphic: scattered secret names (API_KEY, .env, JWT_SECRET…) converging
 * into one central vault — the product's premise as one diagram. Pure markup +
 * CSS keyframes (globals.css); the converge/pop/glow only run once <html> gets
 * `is-ready`, so this renders static and complete without JS. Decorative: the
 * hero copy carries the meaning for assistive tech, so the stage is aria-hidden.
 */

type Chip = { label: string; style: CSSProperties; accent?: boolean };

const CHIPS: Chip[] = [
  { label: 'API_KEY', style: { top: '5%', left: '4%', '--fx': '-46px', '--fy': '-30px', '--d': '0.05s' } as CSSProperties },
  { label: 'DB_PASSWORD', style: { top: '1%', left: '55%', '--fx': '22px', '--fy': '-46px', '--d': '0.12s' } as CSSProperties },
  { label: 'ACCESS_TOKEN', style: { top: '27%', left: '75%', '--fx': '50px', '--fy': '-14px', '--d': '0.19s' } as CSSProperties },
  { label: '.env', accent: true, style: { top: '49%', left: '-1%', '--fx': '-54px', '--fy': '0', '--d': '0.26s' } as CSSProperties },
  { label: 'STRIPE_SECRET', style: { bottom: '20%', left: '78%', '--fx': '52px', '--fy': '26px', '--d': '0.16s' } as CSSProperties },
  { label: 'JWT_SECRET', style: { bottom: '3%', left: '6%', '--fx': '-36px', '--fy': '36px', '--d': '0.22s' } as CSSProperties },
  { label: 'SMTP_PASS', style: { bottom: '0%', left: '56%', '--fx': '24px', '--fy': '46px', '--d': '0.3s' } as CSSProperties },
];

export function HeroStage() {
  return (
    <div className="hero-stage" aria-hidden>
      <div className="hero-ring" />
      <div className="hero-ring r2" />

      {CHIPS.map((chip) => (
        <div key={chip.label} className="hero-chip" style={chip.style}>
          <span
            className="dot"
            style={chip.accent ? { background: 'var(--color-info)' } : undefined}
          />
          {chip.label}
        </div>
      ))}

      <div className="hero-vault-glow" />
      <div className="hero-vault">
        <svg width="34" height="34" viewBox="0 0 22 22" fill="none" aria-hidden className="mx-auto text-text">
          <rect
            x="1"
            y="1"
            width="20"
            height="20"
            rx="5"
            fill="var(--color-surface)"
            stroke="var(--color-border-strong)"
            strokeWidth="1.25"
          />
          <path
            d="M8 6.5 5.5 11 8 15.5M14 6.5 16.5 11 14 15.5"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <circle cx="11" cy="11" r="1.7" fill="var(--color-info)" />
        </svg>
        <div className="mt-2.5 text-md font-semibold tracking-[-0.01em]">Comax</div>
        <div className="mt-0.5 text-xs text-muted">모든 값을 한곳에</div>
        <div className="mt-3 inline-flex items-center gap-1.5 rounded-full bg-success-soft px-2.5 py-[3px] text-xs font-semibold text-success-strong">
          <span className="h-1.5 w-1.5 rounded-full bg-success" />
          암호화 보관
        </div>
      </div>
    </div>
  );
}
