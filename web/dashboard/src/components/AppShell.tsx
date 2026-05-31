import { useState, type ReactNode } from 'react';
import { Button, Container, Flex, Heading, Separator, Text } from '@radix-ui/themes';
import { Link, useRouter } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { logout } from '../lib/auth';

/**
 * Shell for every authenticated route — header with brand + breadcrumb +
 * logout, then renders children below a separator. Keeps the back/up
 * navigation surface in one place so individual pages stay focused on
 * their own data.
 *
 * Breadcrumbs accept Crumb[] rather than rendering Links from a path
 * string because Project / Env names can legitimately contain dots and
 * dashes — re-parsing them out of pathname would be fragile.
 */

export interface Crumb {
  label: string;
  to?: string;
  params?: Record<string, string>;
}

interface AppShellProps {
  crumbs?: Crumb[];
  actions?: ReactNode;
  children: ReactNode;
}

export function AppShell({ crumbs, actions, children }: AppShellProps) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onLogout() {
    setError(null);
    setBusy(true);
    try {
      await logout();
    } catch (err) {
      // Surface the error inline and stay on the page. If we navigated
      // in finally the AppShell unmounts and the error message never
      // renders — the operator would assume the server-side session
      // was revoked when it may not have been.
      setError(err instanceof ApiError ? err.message : 'Logout failed.');
      setBusy(false);
      return;
    }
    setBusy(false);
    await router.navigate({ to: '/login', replace: true });
  }

  return (
    <Container size="4" py="6">
      <Flex direction="column" gap="4">
        <Flex align="center" justify="between" gap="4" wrap="wrap">
          <Flex align="center" gap="3" wrap="wrap">
            <Link
              to="/"
              style={{ textDecoration: 'none', color: 'inherit' }}
              aria-label="Comax Secrets 홈"
            >
              <Heading size="5">Comax Secrets</Heading>
            </Link>
            {crumbs && crumbs.length > 0 ? (
              <nav aria-label="현재 위치">
                <Flex align="center" gap="2">
                  {crumbs.map((crumb, idx) => {
                    const isLast = idx === crumbs.length - 1;
                    return (
                      <Flex key={`${crumb.label}-${idx}`} align="center" gap="2">
                        <Text color="gray" size="2" aria-hidden="true">
                          /
                        </Text>
                        {crumb.to && !isLast ? (
                          <Link
                            to={crumb.to}
                            params={crumb.params}
                            style={{ color: 'var(--accent-11)' }}
                          >
                            {crumb.label}
                          </Link>
                        ) : (
                          <Text size="2" aria-current={isLast ? 'page' : undefined}>
                            {crumb.label}
                          </Text>
                        )}
                      </Flex>
                    );
                  })}
                </Flex>
              </nav>
            ) : null}
          </Flex>
          <Flex align="center" gap="3">
            <Link
              to="/audit"
              style={{ color: 'var(--accent-11)', textDecoration: 'none' }}
              aria-label="감사 로그 보기"
            >
              감사 로그
            </Link>
            {actions}
            <Button
              variant="soft"
              color="gray"
              onClick={onLogout}
              disabled={busy}
              aria-label="로그아웃"
            >
              {busy ? '로그아웃 중…' : '로그아웃'}
            </Button>
          </Flex>
        </Flex>
        <Separator size="4" />
        {error ? (
          <Text role="alert" color="red" size="2">
            {error}
          </Text>
        ) : null}
        {children}
      </Flex>
    </Container>
  );
}
