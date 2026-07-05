import Link from 'next/link';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

type Variant = 'primary' | 'secondary' | 'oncolor' | 'ghost';

const base =
  'inline-flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-semibold transition-colors duration-fast focus-visible:outline-2';

const variants: Record<Variant, string> = {
  // Primary is the Committed teal brand fill (redesign): the landing's one
  // decisive color moment. Near-white brand-text on teal, contrast 5.15:1.
  primary: 'bg-brand text-brand-text hover:bg-brand-hover',
  secondary: 'border border-border-strong text-text hover:bg-surface-hover',
  // Inside a filled teal band: inverted (near-white fill, teal-strong text).
  oncolor: 'bg-brand-text text-brand-strong hover:opacity-90',
  // Inside a filled teal band: outlined ghost.
  ghost: 'border border-white/30 text-brand-text hover:bg-white/10',
};

type ButtonLinkProps = {
  href: string;
  variant?: Variant;
  external?: boolean;
} & Omit<ComponentProps<typeof Link>, 'href'>;

export function ButtonLink({
  href,
  variant = 'primary',
  external,
  className,
  children,
  ...props
}: ButtonLinkProps) {
  const classes = cn(base, variants[variant], className);
  if (external) {
    return (
      <a href={href} target="_blank" rel="noreferrer noopener" className={classes}>
        {children}
      </a>
    );
  }
  return (
    <Link href={href} className={classes} {...props}>
      {children}
    </Link>
  );
}
