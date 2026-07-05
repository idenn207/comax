import { useEffect, useRef, type ReactNode } from 'react';

type AlertVariant = 'form' | 'page';

interface AlertProps {
  variant: AlertVariant;
  message: string | null;
  children?: ReactNode;
}

/**
 * Surface-agnostic alert primitive.
 *
 * Two variants, one ARIA contract (role="alert").
 *
 *   - variant="form"  Used inside dialogs and inline form rows. Pairs
 *     role="alert" with tabIndex={-1} + ref.focus() on mount so the
 *     submitter's keyboard focus follows the screen-reader announcement.
 *     Pulling focus here is the right move because the operator just hit
 *     submit and is waiting for the next instruction; landing them on
 *     the alert is faster than asking them to hunt for it.
 *
 *   - variant="page"  Used on pages, drawers, and banners outside a
 *     dialog scope. role="alert" announces, but focus stays where the
 *     operator put it. Yanking focus on a transient page error would
 *     pull them out of typing or scrolling. Optional `children` slot
 *     for an inline action (retry, dismiss).
 *
 * Replaces FormAlert + cmdk-banner + assorted ad-hoc `<Text role="alert">`
 * / `<Callout color="red" role="alert">` / `<div role="alert">` sites
 * found by /impeccable critique 2026-06-01 (3회차) P1 — five flavors of
 * "this failed" across the surface, each with a different focus and
 * announcement policy. One primitive, one contract.
 *
 * Visual treatment lives in globals.css (.alert-form / .alert-page) so
 * tone tweaks stay in one place. The two variants share color tokens
 * (--color-danger-soft / --color-danger-strong) and diverge only in
 * weight: alert-form is a single line of red text matching field-level
 * vocabulary; alert-page is a soft-red bordered surface.
 */
export function Alert({ variant, message, children }: AlertProps) {
  if (variant === 'form') {
    return <FormAlert message={message} />;
  }
  return <PageAlert message={message}>{children}</PageAlert>;
}

function FormAlert({ message }: { message: string | null }) {
  const ref = useRef<HTMLParagraphElement | null>(null);

  useEffect(() => {
    if (message) {
      ref.current?.focus();
    }
  }, [message]);

  if (!message) return null;

  return (
    <p ref={ref} role="alert" className="alert-form" tabIndex={-1}>
      {message}
    </p>
  );
}

function PageAlert({ message, children }: { message: string | null; children?: ReactNode }) {
  if (!message) return null;

  return (
    <div role="alert" className="alert-page">
      <span className="alert-page-message">{message}</span>
      {children}
    </div>
  );
}
