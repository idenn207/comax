import { useState } from 'react';
import { Button, Card, Container, Flex, Heading, Text } from '@radix-ui/themes';
import { useRouter } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { logout } from '../lib/auth';

/**
 * Authenticated landing page. Task 7 replaces this body with the
 * projects list + bento overview; today it proves the protected route
 * mounts only when logged in and that logout walks the operator back to
 * /login cleanly.
 */
export function HomePage() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onLogout() {
    setError(null);
    setBusy(true);
    try {
      await logout();
    } catch (err) {
      // Surface the error but still let the redirect happen — auth.ts
      // has already cleared local state, so staying on / would just
      // bounce through the guard.
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError('Logout failed.');
      }
    } finally {
      setBusy(false);
      await router.navigate({ to: '/login', replace: true });
    }
  }

  return (
    <Container size="3" py="9">
      <Flex direction="column" gap="4">
        <Flex align="center" justify="between" gap="4">
          <Heading size="8">Comax Secrets</Heading>
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
        <Text color="gray" size="4">
          Dashboard scaffold — Task 7부터 프로젝트/환경/시크릿 화면이 들어옵니다.
        </Text>
        <Card variant="surface">
          <Flex direction="column" gap="2">
            <Text weight="medium">Foundation 상태</Text>
            <Text color="gray" size="2">
              SPA shell가 secret-server에서 정상적으로 서빙됩니다. CSP nonce도 같은 응답에 포함되어
              있습니다.
            </Text>
          </Flex>
        </Card>
        {error ? (
          <Text role="alert" color="red" size="2">
            {error}
          </Text>
        ) : null}
      </Flex>
    </Container>
  );
}
