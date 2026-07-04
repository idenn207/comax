import { cn } from '@/lib/cn';

/**
 * Wordmark. The glyph is a monochrome bracket-key motif (a secret store), not a
 * lock cliché: two brackets enclosing a filled node. Uses currentColor so it
 * inherits ink in both themes.
 */
export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2 font-sans font-semibold', className)}>
      <svg
        width="22"
        height="22"
        viewBox="0 0 22 22"
        fill="none"
        aria-hidden
        className="text-text"
      >
        <rect
          x="1"
          y="1"
          width="20"
          height="20"
          rx="5"
          className="fill-surface-elevated stroke-border-strong"
          strokeWidth="1.25"
        />
        <path
          d="M8 6.5 5.5 11 8 15.5M14 6.5 16.5 11 14 15.5"
          className="stroke-text"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <circle cx="11" cy="11" r="1.6" className="fill-brand" />
      </svg>
      <span className="tracking-[-0.01em]">Comax Secrets</span>
    </span>
  );
}
