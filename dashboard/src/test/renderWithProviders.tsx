import type { ReactElement } from 'react';
import { Theme } from '@radix-ui/themes';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, type RenderOptions, type RenderResult } from '@testing-library/react';

import { ToastProvider } from '../components/Toast';

/**
 * RTL render helper that mirrors main.tsx's provider stack:
 *   Theme → QueryClient → ToastProvider → component
 *
 * A fresh QueryClient is constructed per call so cache state never bleeds
 * between tests. retry is disabled because tests assert the failure path
 * synchronously and don't want to wait for backoff.
 *
 * Pages that hit TanStack Router's useRouter / useParams hooks should
 * mock those hooks directly (see Login.test.tsx) rather than mount the
 * full router — the route guards short-circuit on unauthenticated state
 * and would obscure the actual component being tested.
 */
export function renderWithProviders(
  ui: ReactElement,
  options?: RenderOptions & { queryClient?: QueryClient },
): RenderResult & { queryClient: QueryClient } {
  const queryClient =
    options?.queryClient ??
    new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0, staleTime: 0 },
        mutations: { retry: false },
      },
    });

  const result = render(
    <Theme appearance="dark" accentColor="indigo" grayColor="slate" radius="medium">
      <QueryClientProvider client={queryClient}>
        <ToastProvider>{ui}</ToastProvider>
      </QueryClientProvider>
    </Theme>,
    options,
  );

  return Object.assign(result, { queryClient });
}
