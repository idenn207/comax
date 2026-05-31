import '@radix-ui/themes/styles.css';
import './styles/globals.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { Theme } from '@radix-ui/themes';

import { router } from './router';

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

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('root element missing — index.html should expose <div id="root">');
}

createRoot(rootEl).render(
  <StrictMode>
    <Theme appearance="dark" accentColor="indigo" grayColor="slate" radius="medium">
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </Theme>
  </StrictMode>,
);
