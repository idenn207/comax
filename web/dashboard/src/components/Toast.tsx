import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';

/**
 * Transient notification surface.
 *
 * The toasts live in a bottom-right column and carry only a string + a kind.
 * We deliberately do not ship a full Sonner-style API — the dashboard
 * surfaces success/error in two places:
 *   - mutations (this provider)
 *   - inline form/page alerts (Alert primitive)
 * Anything richer (action buttons, undo) belongs to a future milestone.
 *
 * Dismiss policy is intent-aware (TOAST_DURATION):
 *   - success auto-dismisses after 4s. The operator already saw the
 *     confirmation in the page state; the toast is a footnote.
 *   - error never auto-dismisses. A failed delete shown for 4s while
 *     the operator is mid-keystroke is a missed signal. The toast stays
 *     until the operator acknowledges it via the close button.
 *
 * The stack caps at STACK_MAX with FIFO push-out so consecutive failures
 * don't bury the viewport. Manual close (X) and FIFO eviction both go
 * through dismiss() so their timers, if any, are cleared together.
 *
 * Visual treatment is a token-driven shell (.toast / .toast-success /
 * .toast-danger in globals.css) rather than Radix Callout. The previous
 * implementation passed `highContrast` to Callout, which lifted Radix's
 * own color scale ABOVE our monochrome system — success toasts read as
 * "celebration" instead of "confirm". Keeping the shell in our CSS
 * layer means PRODUCT.md's "조용함" personality is enforced at the design
 * system, not at every call site.
 *
 * aria-live="polite" lets screen readers announce success without
 * interrupting the operator mid-keystroke; errors set role="alert" so
 * they take precedence per WCAG live-region guidance.
 */

type ToastKind = 'success' | 'error';

interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
}

interface ToastContextValue {
  notify: (kind: ToastKind, message: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

// Intent-aware auto-dismiss policy. 0 disables the timer; the operator
// dismisses the toast via the close button. See module docstring above.
const TOAST_DURATION: Record<ToastKind, number> = {
  success: 4_000,
  error: 0,
};

// Cap the visible stack so consecutive errors don't pile up off-screen.
// Three is the smallest stack that lets two errors coexist with a final
// success confirmation (typical retry → succeeds shape).
const STACK_MAX = 3;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const counter = useRef(0);
  // Per-toast auto-dismiss timers, keyed by toast id. Map (not Set) so
  // manual dismiss + FIFO eviction can both look up and clear the timer
  // before removing the toast — no orphaned setTimeout firing into a
  // toast that's already gone.
  const timers = useRef<Map<number, number>>(new Map());

  const clearTimer = useCallback((id: number) => {
    const handle = timers.current.get(id);
    if (handle !== undefined) {
      window.clearTimeout(handle);
      timers.current.delete(id);
    }
  }, []);

  const dismiss = useCallback(
    (id: number) => {
      clearTimer(id);
      setToasts((prev) => prev.filter((t) => t.id !== id));
    },
    [clearTimer],
  );

  const notify = useCallback(
    (kind: ToastKind, message: string) => {
      counter.current += 1;
      const id = counter.current;
      setToasts((prev) => {
        const next = [...prev, { id, kind, message }];
        if (next.length <= STACK_MAX) return next;
        // FIFO push-out. Clear the evicted toast's timer (if any) so it
        // doesn't fire after the toast is gone.
        const overflow = next.length - STACK_MAX;
        for (let i = 0; i < overflow; i += 1) clearTimer(next[i].id);
        return next.slice(overflow);
      });
      const duration = TOAST_DURATION[kind];
      if (duration > 0) {
        const handle = window.setTimeout(() => {
          timers.current.delete(id);
          dismiss(id);
        }, duration);
        timers.current.set(id, handle);
      }
    },
    [clearTimer, dismiss],
  );

  useEffect(() => {
    const active = timers.current;
    return () => {
      for (const handle of active.values()) window.clearTimeout(handle);
      active.clear();
    };
  }, []);

  const value = useMemo(() => ({ notify }), [notify]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-stack" aria-live="polite" aria-atomic="false">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`toast ${toast.kind === 'success' ? 'toast-success' : 'toast-danger'}`}
            role={toast.kind === 'error' ? 'alert' : 'status'}
          >
            <span className="toast-icon" aria-hidden="true">
              {toast.kind === 'success' ? <SuccessGlyph /> : <ErrorGlyph />}
            </span>
            <span className="toast-text">{toast.message}</span>
            {toast.kind === 'error' ? (
              <button
                type="button"
                className="toast-close"
                onClick={() => dismiss(toast.id)}
                aria-label="알림 닫기"
              >
                <CloseGlyph />
              </button>
            ) : null}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function SuccessGlyph() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="m5 12 4.5 4.5L19 7"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ErrorGlyph() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.8" />
      <path d="M12 7v6M12 16v.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

function CloseGlyph() {
  return (
    <svg width="12" height="12" viewBox="0 0 14 14" fill="none" aria-hidden="true">
      <path d="M3 3l8 8M11 3l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    // Fallback: in unit tests that mount a component without ToastProvider,
    // log to the console so the test output still shows what the UI tried
    // to surface. Production never hits this path because main.tsx wraps
    // the entire app.
    return {
      notify: () => {
        /* no-op */
      },
    };
  }
  return ctx;
}
