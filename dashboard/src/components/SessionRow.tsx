import { Badge, Button } from '@radix-ui/themes';

import type { Session } from '../lib/types';

/**
 * One row of the Sessions table.
 *
 * Typographic hierarchy (Design Critique P1 #1, P2 #5):
 *   - device label (parsed UA): font-weight 600, primary read.
 *   - IP prefix: secondary, monospaced for muscle-memory match.
 *   - "Created" timestamp: tertiary, "Created" wording — not "Last
 *     activity" — because the M2 schema does not yet track last_used_at
 *     for sessions; mislabeling would be a lie (DESIGN.md "정직함").
 *
 * Current session (Design Critique P1 #2):
 *   - elevated surface + stronger border, NOT a saturated accent color
 *     (DESIGN.md 원칙 1: 색은 의미에만).
 *   - "현재 세션" chip + revoke button disabled. Revoking the active
 *     session would land the operator on /login mid-action — see the
 *     IsCurrent computation in handlers_sessions.go.
 *
 * Revoke button label is intentionally plain — the destructive warning
 * lives in the ConfirmDialog the page wraps around onRevoke, not on the
 * button itself (Design Critique P1 #3, DESIGN.md 원칙 4).
 */

interface SessionRowProps {
  session: Session;
  onRevoke: (id: number) => void;
  busy: boolean;
}

export function SessionRow({ session, onRevoke, busy }: SessionRowProps) {
  const device = parseDeviceLabel(session.user_agent);
  const created = formatCreatedAt(session.created_at);

  return (
    <tr
      className={`session-row ${session.is_current ? 'session-row--current' : ''}`}
      data-testid={`session-row-${session.id}`}
    >
      <td className="session-row__device">
        <div className="session-row__device-label">{device}</div>
        {session.is_current ? (
          <Badge color="gray" variant="soft" radius="full" size="1" ml="2">
            현재 세션
          </Badge>
        ) : null}
      </td>
      <td className="session-row__ip">
        <code>{session.ip_prefix || '—'}</code>
      </td>
      <td className="session-row__created">
        <time dateTime={session.created_at}>{created}</time>
      </td>
      <td className="session-row__actions">
        <Button
          type="button"
          color="red"
          variant="soft"
          size="1"
          disabled={session.is_current || busy}
          onClick={() => onRevoke(session.id)}
          aria-label={
            session.is_current
              ? '현재 세션은 회수할 수 없습니다 (로그아웃을 사용하세요)'
              : `${device} 세션 회수`
          }
        >
          회수
        </Button>
      </td>
    </tr>
  );
}

/**
 * Tiny self-contained UA parser. ua-parser-js would add ~30 KB to the
 * bundle for the four labels we actually display; a hand-rolled match
 * stays under 1 KB and gives us "Chrome on Windows" granularity, which
 * is what an operator needs to recognise their own laptop.
 *
 * Fallback is "Unknown device" so the row always has something readable
 * (DESIGN.md 원칙 4: 다음 액션 추측 가능).
 *
 * Exported (with eslint-disable for fast-refresh) so Sessions.tsx can
 * render the same parsed label inside the ConfirmDialog without
 * duplicating the parser. Hoisting the parser into a separate module
 * would gain nothing — it's only ever paired with SessionRow.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function parseDeviceLabel(ua: string): string {
  if (!ua) return 'Unknown device';

  let browser = '';
  if (ua.includes('Edg/')) browser = 'Edge';
  else if (ua.includes('OPR/') || ua.includes('Opera')) browser = 'Opera';
  else if (ua.includes('Chrome/')) browser = 'Chrome';
  else if (ua.includes('Firefox/')) browser = 'Firefox';
  else if (ua.includes('Safari/')) browser = 'Safari';

  let os = '';
  if (ua.includes('Windows NT')) os = 'Windows';
  else if (ua.includes('Mac OS X') || ua.includes('Macintosh')) os = 'macOS';
  else if (ua.includes('iPhone') || ua.includes('iPad')) os = 'iOS';
  else if (ua.includes('Android')) os = 'Android';
  else if (ua.includes('Linux')) os = 'Linux';

  if (browser && os) return `${browser} on ${os}`;
  if (browser) return browser;
  if (os) return os;
  return 'Unknown device';
}

function formatCreatedAt(iso: string): string {
  try {
    return new Intl.DateTimeFormat('ko-KR', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}
