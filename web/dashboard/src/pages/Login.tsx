import { useState, type FormEvent } from 'react';
import {
  Box,
  Button,
  Callout,
  Card,
  Container,
  Flex,
  Heading,
  Text,
  TextArea,
} from '@radix-ui/themes';
import { useRouter } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { login } from '../lib/auth';

/**
 * Login form: paste a service token → POST /api/v1/dashboard/session →
 * cookie + CSRF land → navigate to /. Any error stays on this page so
 * the operator can correct it without losing the typed/pasted token.
 *
 * Why a TextArea instead of an input?
 *   Service tokens are long (HKDF base64). A single-line input forces
 *   horizontal scrolling that hides characters; an autosizing textarea
 *   keeps the whole token visible so the operator can spot truncation.
 */
export function LoginPage() {
  const router = useRouter();
  const [token, setToken] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await login(token);
      await router.navigate({ to: '/', replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(formatLoginError(err));
      } else {
        setError('Unexpected error. Please try again later.');
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <Container size="2" py="9">
      <Flex direction="column" gap="4">
        <Heading size="6">로그인</Heading>
        <Card variant="surface">
          <form onSubmit={onSubmit} aria-describedby="login-help">
            <Flex direction="column" gap="3">
              <Box>
                <Text as="label" size="2" weight="medium" htmlFor="token">
                  서비스 토큰
                </Text>
                <Text id="login-help" as="p" size="2" color="gray" mt="1">
                  CLI에서 발급받은 토큰을 그대로 붙여넣으면 dashboard 세션 쿠키 + CSRF
                  토큰을 발급받고 홈으로 이동합니다.
                </Text>
              </Box>
              <TextArea
                id="token"
                name="token"
                placeholder="comax_..."
                value={token}
                onChange={(e) => setToken(e.target.value)}
                rows={3}
                spellCheck={false}
                autoCapitalize="off"
                autoCorrect="off"
                required
                disabled={pending}
                aria-invalid={error !== null}
                aria-errormessage={error ? 'login-error' : undefined}
              />
              {error ? (
                <Callout.Root color="red" role="alert" id="login-error">
                  <Callout.Text>{error}</Callout.Text>
                </Callout.Root>
              ) : null}
              <Flex justify="end">
                <Button type="submit" disabled={pending || token.trim() === ''}>
                  {pending ? '로그인 중…' : '로그인'}
                </Button>
              </Flex>
            </Flex>
          </form>
        </Card>
      </Flex>
    </Container>
  );
}

function formatLoginError(err: ApiError): string {
  switch (err.code) {
    case 'unknown_token':
    case 'missing_bearer':
      return 'Invalid token. Please check and try again.';
    case 'bad_request':
      return err.message;
    case 'network':
      return 'Cannot reach the server. Check the network and secret-server status.';
    default:
      return err.message || 'Login failed.';
  }
}
