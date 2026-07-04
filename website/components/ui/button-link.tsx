import Link from 'next/link';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/cn';

type Variant = 'primary' | 'secondary';

const base =
  'inline-flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-semibold transition-colors duration-fast focus-visible:outline-2';

const variants: Record<Variant, string> = {
  // Primary is monochrome graphite filled — hierarchy by contrast, not hue
  // (mirrors the dashboard; PRODUCT.md "색은 의미에만").
  primary: 'bg-accent text-accent-text hover:bg-accent-hover',
  secondary:
    'border border-border-strong text-text hover:bg-surface-hover',
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
