import { cn } from '@/lib/cn';

export type CommandLine =
  | { kind: 'command'; text: string }
  | { kind: 'comment'; text: string }
  | { kind: 'output'; text: string };

/**
 * A restrained terminal/code card used as the landing's "product imagery"
 * (real commands, not stock photos). Not a CRT-cosplay terminal: it reads like
 * a GitHub code card with a labelled header bar. Color stays monochrome; the
 * one exception is the `$` prompt hint in muted, never a chromatic drench.
 */
export function CommandBlock({
  label,
  lines,
  className,
}: {
  label: string;
  lines: CommandLine[];
  className?: string;
}) {
  return (
    <div
      className={cn(
        'overflow-hidden rounded-lg border border-border bg-surface-elevated shadow-md',
        className,
      )}
    >
      <div className="flex items-center gap-2 border-b border-border bg-panel px-4 py-2.5">
        <span className="flex gap-1.5" aria-hidden>
          <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
          <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
          <span className="h-2.5 w-2.5 rounded-full bg-border-strong" />
        </span>
        <span className="ml-1 font-mono text-xs text-muted">{label}</span>
      </div>
      <pre className="overflow-x-auto px-4 py-4 font-mono text-sm leading-relaxed text-code-text">
        <code>
          {lines.map((line, i) => {
            if (line.kind === 'comment') {
              return (
                <div key={i} className="text-text-faint">
                  {line.text}
                </div>
              );
            }
            if (line.kind === 'output') {
              return (
                <div key={i} className="text-text-subtle">
                  {line.text}
                </div>
              );
            }
            return (
              <div key={i} className="text-code-text">
                <span className="select-none text-muted">$ </span>
                {line.text}
              </div>
            );
          })}
        </code>
      </pre>
    </div>
  );
}
