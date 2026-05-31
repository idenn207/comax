import '@radix-ui/themes/styles.css';
import './styles/globals.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { Theme } from '@radix-ui/themes';

import { registerUnauthorizedHandler } from './lib/api';
import { forceLogout } from './lib/auth';
import { router } from './router';
import { ToastProvider } from './components/Toast';

/**
 * QueryClient lives at the app root so caches survive route changes.
 * 30s staleTime matches the dashboard's mutation pattern: optimistic
 * updates are authoritative until the next refetch. retry=1 avoids the
 * default 3 attempts (operators want fast failure on 4xx).
 */
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 5 * 60 * 1000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
});

/**
 * Wire the api.ts unauthorized hook to the auth state machine + the
 * router. Kept here (rather than inside auth.ts) so the low-level
 * modules stay free of router dependencies — useful for unit tests and
 * future SSR / extraction.
 */
registerUnauthorizedHandler(() => {
  forceLogout();
  // navigate is async but we deliberately fire-and-forget — by the time
  // the promise resolves we'd already be re-rendering anyway.
  void router.navigate({ to: '/login', replace: true });
});

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('root element missing — index.html should expose <div id="root">');
}

createRoot(rootEl).render(
  <StrictMode>
    <Theme appearance="dark" accentColor="indigo" grayColor="slate" radius="medium">
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <RouterProvider router={router} />
        </ToastProvider>
      </QueryClientProvider>
    </Theme>
  </StrictMode>,
);
