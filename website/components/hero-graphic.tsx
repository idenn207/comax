/**
 * Hero graphic — the product's reason for existing, as one teal diagram:
 * a single source of truth (secret-server) fanning secrets into every
 * environment, with prod's drift (a missing key) surfaced in amber.
 * Committed teal (brand 202) + a soft atmospheric glow (Vercel-style depth,
 * painted in the brand hue instead of neutral). Decorative: the hero copy
 * carries the meaning for assistive tech, so this is aria-hidden.
 */
export function HeroGraphic() {
  return (
    <svg
      viewBox="0 0 560 470"
      className="h-auto w-full"
      role="presentation"
      aria-hidden
    >
      <defs>
        <radialGradient id="heroGlow" cx="46%" cy="50%" r="62%">
          <stop offset="0%" stopColor="oklch(0.62 0.11 202)" stopOpacity="0.2" />
          <stop offset="55%" stopColor="oklch(0.62 0.11 202)" stopOpacity="0.05" />
          <stop offset="100%" stopColor="oklch(0.62 0.11 202)" stopOpacity="0" />
        </radialGradient>
      </defs>
      <rect width="560" height="470" fill="url(#heroGlow)" />

      {/* connectors: a faint base line + an animated dashed "flow" overlay */}
      <g className="fill-none stroke-brand opacity-50" strokeWidth={1.6}>
        <path d="M188 235 C 268 235 262 96 344 96" />
        <path d="M188 235 C 268 235 268 235 344 235" />
        <path d="M188 235 C 268 235 262 374 344 374" />
      </g>
      <g className="flow-line fill-none stroke-brand" strokeWidth={1.6}>
        <path d="M188 235 C 268 235 262 96 344 96" />
        <path d="M188 235 C 268 235 268 235 344 235" />
        <path d="M188 235 C 268 235 262 374 344 374" />
      </g>

      {/* source: single source of truth */}
      <g>
        <rect x={30} y={188} width={158} height={94} rx={12} className="fill-brand" />
        <rect
          x={46}
          y={206}
          width={20}
          height={20}
          rx={5}
          className="fill-none stroke-brand-text"
          strokeWidth={1.6}
        />
        <path
          d="M50 206 v-4 a6 6 0 0 1 12 0 v4"
          className="fill-none stroke-brand-text"
          strokeWidth={1.6}
        />
        <text x={74} y={221} className="fill-brand-text font-sans" fontSize={15} fontWeight={700}>
          secret-server
        </text>
        <text x={46} y={252} className="fill-brand-text font-mono" fontSize={12.5} opacity={0.82}>
          secrets.db · SQLite
        </text>
        <text x={46} y={270} className="fill-brand-text font-mono" fontSize={12.5} opacity={0.82}>
          진실의 출처
        </text>
      </g>

      {/* dest: worktree / dev */}
      <g>
        <rect x={344} y={58} width={192} height={76} rx={11} className="fill-surface-elevated stroke-border" />
        <text x={360} y={82} className="fill-muted font-mono" fontSize={12}>
          worktree · dev
        </text>
        <circle cx={366} cy={104} r={3.2} className="fill-success" />
        <text x={378} y={108} className="fill-text font-mono" fontSize={12.5}>
          14 secrets 주입됨
        </text>
      </g>

      {/* dest: GitHub Actions / CI */}
      <g>
        <rect x={344} y={197} width={192} height={76} rx={11} className="fill-surface-elevated stroke-border" />
        <text x={360} y={221} className="fill-muted font-mono" fontSize={12}>
          GitHub Actions · CI
        </text>
        <circle cx={366} cy={243} r={3.2} className="fill-success" />
        <text x={378} y={247} className="fill-text font-mono" fontSize={12.5}>
          step env 자동 주입
        </text>
      </g>

      {/* dest: prod — drift surfaced */}
      <g>
        <rect x={344} y={336} width={192} height={76} rx={11} className="fill-surface-elevated stroke-border-strong" />
        <text x={360} y={360} className="fill-muted font-mono" fontSize={12}>
          prod
        </text>
        <rect x={360} y={374} width={168} height={24} rx={6} className="fill-warning-soft" />
        <text
          x={371}
          y={390}
          className="fill-[var(--color-warning-strong)] font-mono"
          fontSize={12.5}
          fontWeight={600}
        >
          ! STRIPE_KEY 누락
        </text>
      </g>
    </svg>
  );
}
