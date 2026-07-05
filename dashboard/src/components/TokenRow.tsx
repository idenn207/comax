import { Badge, Button } from '@radix-ui/themes';

import type { Token } from '../lib/types';

/**
 * One row of the Tokens table. Mirrors SessionRow's structure and reuses
 * the same .session-row / .sessions-table CSS so the two operator tables
 * stay visually identical (GitHub Settings → Tokens is the shared anchor).
 *
 * Typographic hierarchy:
 *   - token name: primary read, monospaced (names like `ci-github` read
 *     as identifiers).
 *   - admin badge: gray soft, NOT a colored accent — admin is neither
 *     danger nor success, so per DESIGN.md 원칙 1 (색은 의미에만) it stays
 *     monochrome; only status/focus earn color.
 *   - created / last-used: tertiary timestamps. "—" when never used, so
 *     an absent last-used reads as a fact, not a blank (정직함).
 *
 * Revoked tokens: a gray "회수됨" badge replaces the revoke button. Revoked
 * is inactive, not an error, so the signal is a label (badge), not color —
 * red is reserved for the destructive action itself (DESIGN.md: 색만으로
 * 상태 전달 금지).
 */

interface TokenRowProps {
  token: Token;
  onRevoke: (id: number) => void;
  busy: boolean;
}

export function TokenRow({ token, onRevoke, busy }: TokenRowProps) {
  const revoked = Boolean(token.revoked_at);

  return (
    <tr className="session-row" data-testid={`token-row-${token.id}`}>
      <td className="session-row__device">
        <code className="session-row__device-label">{token.name}</code>
        {token.is_admin ? (
          <Badge color="gray" variant="soft" radius="full" size="1" ml="2">
            admin
          </Badge>
        ) : null}
      </td>
      <td className="session-row__created">
        <time dateTime={token.created_at}>{formatDate(token.created_at)}</time>
      </td>
      <td className="session-row__created">
        {token.last_used_at ? (
          <time dateTime={token.last_used_at}>{formatDate(token.last_used_at)}</time>
        ) : (
          <span className="text-muted">—</span>
        )}
      </td>
      <td className="session-row__actions">
        {revoked ? (
          <Badge color="gray" variant="soft" radius="full" size="1">
            회수됨
          </Badge>
        ) : (
          <Button
            type="button"
            color="red"
            variant="soft"
            size="1"
            disabled={busy}
            onClick={() => onRevoke(token.id)}
            aria-label={`토큰 ${token.name} 회수`}
          >
            회수
          </Button>
        )}
      </td>
    </tr>
  );
}

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat('ko-KR', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}
