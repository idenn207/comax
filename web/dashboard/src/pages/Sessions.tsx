import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Alert } from '../components/Alert';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { PageHeader } from '../components/PageHeader';
import { SessionRow, parseDeviceLabel } from '../components/SessionRow';
import { AppShell } from '../components/AppShell';
import { ApiError } from '../lib/api';
import { listSessions, queryKeys, revokeSession } from '../lib/queries';

/**
 * Sessions page — `/settings/sessions`.
 *
 * Shape decisions absorbed from the plan's Design Critique:
 *   - Table, not card grid (P1 #1). GitHub Settings → Personal Access
 *     Tokens is the closest vocabulary; PRODUCT.md's anchor.
 *   - Current session: elevated surface + "현재 세션" chip + revoke
 *     disabled. No saturated accent (P1 #2, DESIGN.md 원칙 1).
 *   - ConfirmDialog copy carries the honest "cookie 탈취 후에는 revoke
 *     로 막을 수 없다" warning (P1 #3, threat-model.md Browser sessions
 *     Limit 단락과 의미 일치).
 *   - Device label parsed from UA, never raw (P1 #4).
 *   - "Created" wording (P2 #5) — last_used_at은 M2 schema에 없음.
 *   - Empty state: 1줄, 다른 액션 없음 (P2 #6, DESIGN.md 원칙 4).
 *   - Table → mobile stacked card reflow는 globals.css에서 처리 (P2 #7).
 */
export function SessionsPage() {
  const qc = useQueryClient();
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const sessionsQ = useQuery({
    queryKey: queryKeys.sessions(),
    queryFn: ({ signal }) => listSessions(signal),
  });

  const revoke = useMutation({
    mutationFn: (id: number) => revokeSession(id),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.sessions() });
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Could not revoke session.');
    },
  });

  const pendingSession =
    pendingId !== null ? sessionsQ.data?.find((s) => s.id === pendingId) ?? null : null;

  function startRevoke(id: number) {
    setError(null);
    setPendingId(id);
  }

  async function confirmRevoke() {
    if (pendingId === null) return;
    await revoke.mutateAsync(pendingId);
    setPendingId(null);
  }

  return (
    <AppShell crumbs={[{ label: '설정', to: '/' }, { label: '세션' }]}>
      <PageHeader title="활성 세션" />

      {error ? (
        <div className="mb-4">
          <Alert variant="page" message={error} />
        </div>
      ) : null}

      {sessionsQ.isError ? (
        <Alert
          variant="page"
          message={
            sessionsQ.error instanceof ApiError
              ? sessionsQ.error.message
              : '세션 목록을 불러오지 못했습니다.'
          }
        />
      ) : sessionsQ.isLoading ? (
        <p className="text-muted">불러오는 중…</p>
      ) : !sessionsQ.data || sessionsQ.data.length === 0 ? (
        <p className="text-muted">다른 활성 세션이 없습니다.</p>
      ) : (
        <table className="sessions-table" role="table" aria-label="활성 세션 목록">
          <thead>
            <tr>
              <th scope="col">디바이스</th>
              <th scope="col">IP</th>
              <th scope="col">생성</th>
              <th scope="col" className="sr-only">
                액션
              </th>
            </tr>
          </thead>
          <tbody>
            {sessionsQ.data.map((s) => (
              <SessionRow
                key={s.id}
                session={s}
                onRevoke={startRevoke}
                busy={revoke.isPending && revoke.variables === s.id}
              />
            ))}
          </tbody>
        </table>
      )}

      <ConfirmDialog
        open={pendingId !== null}
        onOpenChange={(open) => {
          if (!open) setPendingId(null);
        }}
        title="세션 회수"
        description={
          <div className="confirm-dialog__body">
            <p>
              <strong>
                {pendingSession ? parseDeviceLabel(pendingSession.user_agent) : '이 세션'}
              </strong>
              의 cookie는 더 이상 인증에 사용되지 않습니다.
            </p>
            <p className="confirm-dialog__caveat">
              다만 cookie가 이미 탈취된 상태였다면 그 시점까지 발생한 read 는 막을 수 없습니다.
              cookie 자체가 의심된다면 해당 token 을 통째로 revoke 하세요.
            </p>
          </div>
        }
        confirmLabel="회수"
        intent="danger"
        onConfirm={confirmRevoke}
      />
    </AppShell>
  );
}
