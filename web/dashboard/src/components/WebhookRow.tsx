import { Badge, Button } from '@radix-ui/themes';

import type { Webhook } from '../lib/types';

/**
 * One row of the Webhooks table. Mirrors TokenRow's structure and reuses the
 * same .session-row / .sessions-table CSS so the operator tables stay visually
 * identical (the Integrations screens share the Tokens vocabulary, GitHub
 * Settings being the shared anchor).
 *
 * Typographic hierarchy:
 *   - scope (project/env): primary read, monospaced — it reads as a coordinate.
 *   - url: secondary, monospaced code.
 *   - events: gray soft badges — labels, not accents (DESIGN.md 원칙: 색은
 *     의미에만). Only the enabled/disabled status earns a color.
 *   - enabled: green "활성" / gray "비활성". Disabled is inactive, not an error,
 *     so it is a gray label, never red — red is reserved for the destructive
 *     action itself.
 */

interface WebhookRowProps {
  webhook: Webhook;
  onDelete: (id: number) => void;
  onToggle: (id: number, enabled: boolean) => void;
  onShowDeliveries: (id: number) => void;
  busy: boolean;
}

export function WebhookRow({ webhook, onDelete, onToggle, onShowDeliveries, busy }: WebhookRowProps) {
  const scope = webhook.env ? `${webhook.project}/${webhook.env}` : `${webhook.project}/*`;

  return (
    <tr className="session-row" data-testid={`webhook-row-${webhook.id}`}>
      <td className="session-row__device">
        <code className="session-row__device-label">{scope}</code>
      </td>
      <td className="session-row__created">
        <code>{webhook.url}</code>
      </td>
      <td className="session-row__created">
        {webhook.events.map((e) => (
          <Badge key={e} color="gray" variant="soft" radius="full" size="1" mr="1">
            {e.replace('secret.', '')}
          </Badge>
        ))}
      </td>
      <td className="session-row__created">
        {webhook.enabled ? (
          <Badge color="green" variant="soft" radius="full" size="1">
            활성
          </Badge>
        ) : (
          <Badge color="gray" variant="soft" radius="full" size="1">
            비활성
          </Badge>
        )}
      </td>
      <td className="session-row__actions">
        <Button
          type="button"
          variant="soft"
          color="gray"
          size="1"
          mr="2"
          onClick={() => onShowDeliveries(webhook.id)}
          aria-label={`웹훅 ${scope} 배달 이력 보기`}
        >
          배달
        </Button>
        <Button
          type="button"
          variant="soft"
          color="gray"
          size="1"
          mr="2"
          disabled={busy}
          onClick={() => onToggle(webhook.id, !webhook.enabled)}
          aria-label={webhook.enabled ? `웹훅 ${scope} 비활성화` : `웹훅 ${scope} 활성화`}
        >
          {webhook.enabled ? '비활성화' : '활성화'}
        </Button>
        <Button
          type="button"
          color="red"
          variant="soft"
          size="1"
          disabled={busy}
          onClick={() => onDelete(webhook.id)}
          aria-label={`웹훅 ${scope} 삭제`}
        >
          삭제
        </Button>
      </td>
    </tr>
  );
}
