import type { ReactNode } from 'react';

/**
 * Action-centric page header. Title on the left, primary actions on the
 * right — the GitHub / Linear pattern that places mutations adjacent to
 * the section they affect, rather than parking them in the top bar.
 *
 * `eyebrow` is reserved for status counts and parent context that the
 * crumb already conveys ("12개 프로젝트", "prod ← staging"). It is NOT
 * a decorative kicker; if you don't have a real value to put there,
 * leave it out. (See impeccable absolute bans: tracked uppercase eyebrow
 * above every section is template scaffolding.)
 *
 * The page-head class lives in globals.css so a future markup swap to
 * <header> with semantic landmarks doesn't lose the layout.
 */

interface PageHeaderProps {
  title: ReactNode;
  eyebrow?: ReactNode;
  actions?: ReactNode;
}

export function PageHeader({ title, eyebrow, actions }: PageHeaderProps) {
  return (
    <div className="page-head">
      <div className="page-head-text">
        {eyebrow ? <span className="page-eyebrow">{eyebrow}</span> : null}
        <h1 className="page-title">{title}</h1>
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </div>
  );
}
