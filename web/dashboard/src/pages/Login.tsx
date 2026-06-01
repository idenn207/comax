import { useState, type FormEvent } from 'react';
import { Button, Callout, Flex, TextArea } from '@radix-ui/themes';
import { useRouter } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { login } from '../lib/auth';

/**
 * Login form: paste a service token → POST /api/v1/dashboard/session →
 * cookie + CSRF land → navigate to /. Any error stays on this page so
 * the operator can correct it without losing the typed/pasted token.
 *
 * No AppShell here: the login surface is single-task and intentionally
 * contained. A small Comax brand mark anchors the panel so the operator
 * sees they're at the right server before pasting a secret token.
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
    <main
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        padding: 24,
        background: 'var(--color-surface)',
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 440,
          display: 'flex',
          flexDirection: 'column',
          gap: 24,
        }}
      >
        <Flex align="center" gap="3">
          <span
            className="nav-brand-mark"
            aria-hidden="true"
            style={{ width: 36, height: 36, fontSize: 16 }}
          >
            C
          </span>
          <Flex direction="column" style={{ lineHeight: 1.1 }}>
            <span
              style={{ fontSize: 'var(--text-lg)', fontWeight: 600, letterSpacing: '-0.015em' }}
            >
              Comax Secrets
            </span>
            <span
              style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--color-muted)',
                letterSpacing: '0.04em',
              }}
            >
              self-hosted
            </span>
          </Flex>
        </Flex>

        <div
          style={{
            background: 'var(--color-surface-elevated)',
            border: '1px solid var(--color-border)',
            borderRadius: 'var(--radius-lg)',
            padding: 28,
            boxShadow: 'var(--shadow-md)',
          }}
        >
          <form onSubmit={onSubmit} aria-describedby="login-help">
            <Flex direction="column" gap="4">
              <Flex direction="column" gap="1">
                <h1
                  style={{
                    margin: 0,
                    fontSize: 'var(--text-xl)',
                    fontWeight: 600,
                    letterSpacing: '-0.015em',
                  }}
                >
                  로그인
                </h1>
                <p
                  id="login-help"
                  style={{
                    margin: 0,
                    fontSize: 'var(--text-sm)',
                    color: 'var(--color-text-subtle)',
                    lineHeight: 'var(--line-snug)',
                  }}
                >
                  CLI에서 발급받은 서비스 토큰을 그대로 붙여넣으면 dashboard 세션 쿠키 + CSRF 토큰을
                  발급받고 홈으로 이동합니다.
                </p>
              </Flex>
              <Flex direction="column" gap="2">
                <label
                  htmlFor="token"
                  style={{
                    fontSize: 'var(--text-sm)',
                    fontWeight: 600,
                    color: 'var(--color-text)',
                  }}
                >
                  서비스 토큰
                </label>
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
              </Flex>
              {error ? (
                <Callout.Root color="red" role="alert" id="login-error">
                  <Callout.Text>{error}</Callout.Text>
                </Callout.Root>
              ) : null}
              <Flex justify="end" mt="1">
                <Button type="submit" disabled={pending || token.trim() === ''}>
                  {pending ? '로그인 중…' : '로그인'}
                </Button>
              </Flex>
            </Flex>
          </form>
        </div>

        <p
          style={{
            margin: 0,
            fontSize: 'var(--text-xs)',
            color: 'var(--color-text-faint)',
            textAlign: 'center',
            lineHeight: 'var(--line-snug)',
          }}
        >
          서버는 자체 호스팅 중입니다. 토큰은 브라우저를 떠나지 않으며 세션 쿠키는 HttpOnly·Secure
          입니다.
        </p>
      </div>
    </main>
  );
}

function formatLoginError(err: ApiError): string {
  switch (err.code) {
    case 'unknown_token':
    case 'missing_bearer':
    case 'unauthorized':
      return 'Invalid token. Please check and try again.';
    case 'bad_request':
      return err.message;
    case 'network':
      return 'Cannot reach the server. Check the network and secret-server status.';
    default:
      return err.message || 'Login failed.';
  }
}
