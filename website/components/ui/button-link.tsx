import Link from 'next/link';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

type Variant = 'primary' | 'outline' | 'soft' | 'ghost';
type Size = 'sm' | 'md' | 'lg';

const base =
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md border font-semibold tracking-[-0.005em] transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus';

const sizes: Record<Size, string> = {
  sm: 'h-[26px] px-2.5 text-xs',
  md: 'h-[34px] px-3.5 text-base',
  lg: 'h-10 px-[18px] text-md',
};

const variants: Record<Variant, string> = {
  // Primary is monochrome graphite filled — hierarchy by contrast, not hue
  // (mirrors the dashboard; "색은 의미에만"). Color lives on links, not actions.
  primary: 'border-accent bg-accent text-accent-text hover:border-accent-hover hover:bg-accent-hover',
  // Outline — hairline border, transparent fill. The default secondary action.
  outline: 'border-border-strong text-text hover:bg-surface-hover',
  // Soft — neutral surface fill, no border.
  soft: 'border-transparent bg-surface-hover text-text hover:bg-surface-active',
  // Ghost — no chrome until hover.
  ghost: 'border-transparent text-text-subtle hover:bg-surface-hover hover:text-text',
};

type ButtonLinkProps = {
  href: string;
  variant?: Variant;
  size?: Size;
  external?: boolean;
} & Omit<ComponentProps<typeof Link>, 'href'>;

export function ButtonLink({
  href,
  variant = 'primary',
  size = 'md',
  external,
  className,
  children,
  ...props
}: ButtonLinkProps) {
  const classes = cn(base, sizes[size], variants[variant], className);
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
