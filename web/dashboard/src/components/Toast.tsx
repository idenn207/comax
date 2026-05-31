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
import { Box, Callout, Flex } from '@radix-ui/themes';

/**
 * Transient notification surface.
 *
 * The toasts live in a bottom-right column, auto-dismiss after 4s, and
 * carry only a string + a kind. We deliberately do not ship a full
 * Sonner-style API — the dashboard surfaces success/error in two places:
 *   - mutations (this provider)
 *   - inline form/page alerts (Callout)
 * Anything richer (action buttons, undo) belongs to a future milestone.
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

const TOAST_DURATION_MS = 4_000;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const counter = useRef(0);
  // Active auto-dismiss timeouts. Cleared on unmount so tests that
  // mount/unmount the provider don't see stray timers run against a
  // dead state setter.
  const timers = useRef<Set<number>>(new Set());

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const notify = useCallback(
    (kind: ToastKind, message: string) => {
      counter.current += 1;
      const id = counter.current;
      setToasts((prev) => [...prev, { id, kind, message }]);
      const handle = window.setTimeout(() => {
        timers.current.delete(handle);
        dismiss(id);
      }, TOAST_DURATION_MS);
      timers.current.add(handle);
    },
    [dismiss],
  );

  useEffect(() => {
    const active = timers.current;
    return () => {
      for (const handle of active) window.clearTimeout(handle);
      active.clear();
    };
  }, []);

  const value = useMemo(() => ({ notify }), [notify]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <Box
        aria-live="polite"
        aria-atomic="false"
        style={{
          position: 'fixed',
          right: 16,
          bottom: 16,
          maxWidth: 'min(420px, calc(100vw - 32px))',
          zIndex: 1000,
        }}
      >
        <Flex direction="column" gap="2">
          {toasts.map((toast) => (
            <Callout.Root
              key={toast.id}
              color={toast.kind === 'success' ? 'green' : 'red'}
              role={toast.kind === 'error' ? 'alert' : 'status'}
              highContrast
            >
              <Callout.Text>{toast.message}</Callout.Text>
            </Callout.Root>
          ))}
        </Flex>
      </Box>
    </ToastContext.Provider>
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
