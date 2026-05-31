import type { ReactElement } from 'react';
import { Theme } from '@radix-ui/themes';
import { render, type RenderOptions, type RenderResult } from '@testing-library/react';

/**
 * RTL render helper that puts the component under the same Radix Theme
 * provider main.tsx uses. Pages that only consume Radix primitives can
 * be tested in isolation this way; pages that hit TanStack Router's
 * useRouter hook should mock the hook directly (see Login.test.tsx).
 */
export function renderWithProviders(
  ui: ReactElement,
  options?: RenderOptions,
): RenderResult {
  return render(
    <Theme appearance="dark" accentColor="indigo" grayColor="slate" radius="medium">
      {ui}
    </Theme>,
    options,
  );
}
