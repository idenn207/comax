import { useState, type FormEvent } from 'react';
import { Badge, Button, Checkbox, Dialog, Flex, Text, TextField } from '@radix-ui/themes';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Alert } from '../components/Alert';
import { AppShell } from '../components/AppShell';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { FormField } from '../components/FormField';
import { PageHeader } from '../components/PageHeader';
import { WebhookRow } from '../components/WebhookRow';
import { ApiError } from '../lib/api';
import {
  createWebhook,
  deleteWebhook,
  listDeliveries,
  listWebhooks,
  queryKeys,
  setWebhookEnabled,
} from '../lib/queries';
import type { CreatedWebhook, WebhookDelivery } from '../lib/types';
import { NAME_FORMAT_HINT, nameError } from '../lib/validate';

/**
 * Webhooks page — `/integrations/webhooks`.
 *
 * Webhook management: register (admin), list, delete, inspect deliveries. A
 * webhook POSTs a signed, metadata-only event to an operator URL when a
 * subscribed secret change commits; the receiver re-pulls the value itself.
 *
 * Mirrors the Tokens page vocabulary (DESIGN.md · GitHub Settings anchor):
 *   - .sessions-table reused verbatim — operator tables stay identical.
 *   - Register flow reveals the signing secret exactly once (two-phase dialog),
 *     since it cannot be re-fetched.
 *   - Delete carries the honest caveat that delivery history goes with it.
 *   - A non-admin session gets 403 on GET /webhooks, surfaced as an "admin
 *     only" notice rather than an error banner (정직함).
 */

const EVENT_OPTIONS = [
  { value: 'secret.upsert', label: '생성·수정' },
  { value: 'secret.rollback', label: '롤백' },
  { value: 'secret.delete', label: '삭제' },
] as const;

export function WebhooksPage() {
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [deliveriesId, setDeliveriesId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const webhooksQ = useQuery({
    queryKey: queryKeys.webhooks(),
    queryFn: ({ signal }) => listWebhooks(signal),
    // Admin-only endpoint: a 403 is an expected outcome for a non-admin
    // session, not a transient failure, so don't retry it.
    retry: false,
  });

  const remove = useMutation({
    mutationFn: (id: number) => deleteWebhook(id),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.webhooks() });
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : '웹훅을 삭제하지 못했습니다.');
    },
  });

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => setWebhookEnabled(id, enabled),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.webhooks() });
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : '웹훅 상태를 변경하지 못했습니다.');
    },
  });

  function toggleEnabled(id: number, enabled: boolean) {
    setError(null);
    toggle.mutate({ id, enabled });
  }

  const forbidden =
    webhooksQ.isError && webhooksQ.error instanceof ApiError && webhooksQ.error.code === 'forbidden';

  const pendingWebhook =
    pendingId !== null ? webhooksQ.data?.find((h) => h.id === pendingId) ?? null : null;

  function startDelete(id: number) {
    setError(null);
    setPendingId(id);
  }

  async function confirmDelete() {
    if (pendingId === null) return;
    await remove.mutateAsync(pendingId);
    setPendingId(null);
  }

  return (
    <AppShell active="webhooks" crumbs={[{ label: '연동', to: '/' }, { label: '웹훅' }]}>
      <PageHeader
        title="웹훅"
        actions={
          forbidden ? undefined : (
            <Button type="button" onClick={() => setCreateOpen(true)}>
              새 웹훅
            </Button>
          )
        }
      />

      {error ? (
        <div className="mb-4">
          <Alert variant="page" message={error} />
        </div>
      ) : null}

      {forbidden ? (
        <p className="text-muted">
          웹훅 관리는 관리자 토큰으로만 가능합니다. 관리자에게 등록을 요청하세요.
        </p>
      ) : webhooksQ.isError ? (
        <Alert
          variant="page"
          message={
            webhooksQ.error instanceof ApiError
              ? webhooksQ.error.message
              : '웹훅 목록을 불러오지 못했습니다.'
          }
        />
      ) : webhooksQ.isLoading ? (
        <p className="text-muted">불러오는 중…</p>
      ) : !webhooksQ.data || webhooksQ.data.length === 0 ? (
        <p className="text-muted">등록된 웹훅이 없습니다.</p>
      ) : (
        <table className="sessions-table" role="table" aria-label="웹훅 목록">
          <thead>
            <tr>
              <th scope="col">대상</th>
              <th scope="col">URL</th>
              <th scope="col">이벤트</th>
              <th scope="col">상태</th>
              <th scope="col" className="sr-only">
                액션
              </th>
            </tr>
          </thead>
          <tbody>
            {webhooksQ.data.map((h) => (
              <WebhookRow
                key={h.id}
                webhook={h}
                onDelete={startDelete}
                onToggle={toggleEnabled}
                onShowDeliveries={setDeliveriesId}
                busy={
                  (remove.isPending && remove.variables === h.id) ||
                  (toggle.isPending && toggle.variables?.id === h.id)
                }
              />
            ))}
          </tbody>
        </table>
      )}

      <CreateWebhookDialog open={createOpen} onOpenChange={setCreateOpen} />

      <DeliveriesDialog
        webhookId={deliveriesId}
        onOpenChange={(open) => {
          if (!open) setDeliveriesId(null);
        }}
      />

      <ConfirmDialog
        open={pendingId !== null}
        onOpenChange={(open) => {
          if (!open) setPendingId(null);
        }}
        title="웹훅 삭제"
        description={
          <div className="confirm-dialog__body">
            <p>
              <strong>{pendingWebhook ? pendingWebhook.url : '이 웹훅'}</strong>이 삭제됩니다. 이후
              이 대상으로는 이벤트가 전송되지 않습니다.
            </p>
            <p className="confirm-dialog__caveat">
              이 웹훅의 배달 이력도 함께 제거되며, 대기 중이던 배달은 중단됩니다. 서명 시크릿을 다시
              쓰려면 새 웹훅을 등록해야 합니다.
            </p>
          </div>
        }
        confirmLabel="삭제"
        intent="danger"
        onConfirm={confirmDelete}
      />
    </AppShell>
  );
}

interface CreateWebhookDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Two-phase register dialog. Phase 1 collects project/env/url/events; phase 2
 * shows the signing secret exactly once (copy button + a warning it cannot be
 * re-fetched). Closing the dialog resets both phases.
 */
function CreateWebhookDialog({ open, onOpenChange }: CreateWebhookDialogProps) {
  const qc = useQueryClient();
  const [project, setProject] = useState('');
  const [env, setEnv] = useState('');
  const [url, setUrl] = useState('');
  const [events, setEvents] = useState<Record<string, boolean>>({
    'secret.upsert': true,
    'secret.rollback': true,
    'secret.delete': true,
  });
  const [fieldError, setFieldError] = useState<Record<string, string | null>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedWebhook | null>(null);
  const [copied, setCopied] = useState(false);

  const mutation = useMutation({
    mutationFn: (input: { project: string; env?: string; url: string; events?: string[] }) =>
      createWebhook(input),
    onSuccess: async (webhook) => {
      await qc.invalidateQueries({ queryKey: queryKeys.webhooks() });
      setCreated(webhook);
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.code === 'forbidden') {
          setFormError('웹훅 등록은 관리자만 가능합니다.');
          return;
        }
        if (err.code === 'not_found') {
          setFormError('프로젝트 또는 환경을 찾을 수 없습니다. 이름을 확인하세요.');
          return;
        }
        setFormError(err.message);
        return;
      }
      setFormError('알 수 없는 오류로 등록에 실패했습니다. 잠시 후 다시 시도해 주세요.');
    },
  });

  function reset() {
    setProject('');
    setEnv('');
    setUrl('');
    setEvents({ 'secret.upsert': true, 'secret.rollback': true, 'secret.delete': true });
    setFieldError({});
    setFormError(null);
    setCreated(null);
    setCopied(false);
    mutation.reset();
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFieldError({});
    setFormError(null);

    const errs: Record<string, string | null> = {};
    const trimmedProject = project.trim();
    errs.project = nameError('project', trimmedProject);
    const trimmedEnv = env.trim();
    if (trimmedEnv !== '') errs.env = nameError('env', trimmedEnv);
    const trimmedUrl = url.trim();
    if (trimmedUrl === '') {
      errs.url = 'URL을 입력하세요.';
    } else if (!/^https?:\/\//i.test(trimmedUrl)) {
      errs.url = 'http:// 또는 https:// 로 시작해야 합니다.';
    }
    const selected = EVENT_OPTIONS.map((o) => o.value).filter((v) => events[v]);
    if (selected.length === 0) {
      errs.events = '이벤트를 하나 이상 선택하세요.';
    }
    if (Object.values(errs).some(Boolean)) {
      setFieldError(errs);
      return;
    }

    mutation.mutate({
      project: trimmedProject,
      env: trimmedEnv === '' ? undefined : trimmedEnv,
      url: trimmedUrl,
      events: selected,
    });
  }

  async function copySecret() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.signing_secret);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="var(--dialog-width-sm)">
        {created ? (
          <>
            <Dialog.Title>웹훅 등록됨</Dialog.Title>
            <Dialog.Description size="2" mb="3">
              아래 서명 시크릿은 <strong>지금 한 번만</strong> 표시됩니다. 복사해서 수신 서버의 서명
              검증(<code>X-Comax-Signature</code>)에 저장하세요. 닫으면 다시 볼 수 없습니다.
            </Dialog.Description>
            <Flex direction="column" gap="3">
              <TextField.Root
                value={created.signing_secret}
                readOnly
                aria-label="서명 시크릿"
                onFocus={(e) => e.currentTarget.select()}
              />
              <Flex gap="3" justify="end">
                <Button type="button" variant="soft" onClick={copySecret}>
                  {copied ? '복사됨' : '복사'}
                </Button>
                <Dialog.Close>
                  <Button type="button">닫기</Button>
                </Dialog.Close>
              </Flex>
            </Flex>
          </>
        ) : (
          <>
            <Dialog.Title>새 웹훅</Dialog.Title>
            <Dialog.Description size="2" mb="3">
              시크릿 변경 시 서명된 이벤트를 받을 URL을 등록합니다. 페이로드에는 메타데이터만 담기며
              시크릿 값은 담기지 않습니다.
            </Dialog.Description>
            <form onSubmit={onSubmit}>
              <Flex direction="column" gap="3">
                <FormField
                  id="create-webhook-project"
                  label="프로젝트"
                  hint={NAME_FORMAT_HINT}
                  error={fieldError.project ?? null}
                >
                  {(fieldProps) => (
                    <TextField.Root
                      {...fieldProps}
                      value={project}
                      onChange={(e) => setProject(e.target.value)}
                      placeholder="예: comax"
                      autoFocus
                      spellCheck={false}
                    />
                  )}
                </FormField>
                <FormField
                  id="create-webhook-env"
                  label="환경 (선택)"
                  hint="비우면 프로젝트의 모든 환경에 적용됩니다."
                  error={fieldError.env ?? null}
                >
                  {(fieldProps) => (
                    <TextField.Root
                      {...fieldProps}
                      value={env}
                      onChange={(e) => setEnv(e.target.value)}
                      placeholder="예: prod"
                      spellCheck={false}
                    />
                  )}
                </FormField>
                <FormField
                  id="create-webhook-url"
                  label="수신 URL"
                  hint="http/https. 내부 서비스 주소를 사용할 수 있습니다."
                  error={fieldError.url ?? null}
                >
                  {(fieldProps) => (
                    <TextField.Root
                      {...fieldProps}
                      value={url}
                      onChange={(e) => setUrl(e.target.value)}
                      placeholder="예: http://deploy.internal/redeploy"
                      spellCheck={false}
                    />
                  )}
                </FormField>
                <div>
                  <Text as="label" size="2" weight="medium">
                    이벤트
                  </Text>
                  <Flex direction="column" gap="1" mt="1">
                    {EVENT_OPTIONS.map((opt) => (
                      <Text as="label" size="2" key={opt.value}>
                        <Flex gap="2" align="center">
                          <Checkbox
                            checked={events[opt.value]}
                            onCheckedChange={(checked) =>
                              setEvents((prev) => ({ ...prev, [opt.value]: checked === true }))
                            }
                          />
                          {opt.label}{' '}
                          <Badge color="gray" variant="soft" radius="full" size="1">
                            {opt.value}
                          </Badge>
                        </Flex>
                      </Text>
                    ))}
                  </Flex>
                  {fieldError.events ? (
                    <Text as="p" size="1" color="red" mt="1">
                      {fieldError.events}
                    </Text>
                  ) : null}
                </div>
                <Alert variant="form" message={formError} />
                <Flex gap="3" justify="end">
                  <Dialog.Close>
                    <Button variant="soft" color="gray" type="button">
                      취소
                    </Button>
                  </Dialog.Close>
                  <Button type="submit" disabled={mutation.isPending || project.trim() === ''}>
                    {mutation.isPending ? '등록 중…' : '등록'}
                  </Button>
                </Flex>
              </Flex>
            </form>
          </>
        )}
      </Dialog.Content>
    </Dialog.Root>
  );
}

interface DeliveriesDialogProps {
  webhookId: number | null;
  onOpenChange: (open: boolean) => void;
}

/**
 * Recent-deliveries dialog. Fetches on open (enabled by a non-null id) and
 * renders the delivery status honestly: delivered / pending / dead, with the
 * last HTTP status and the failure reason when present.
 */
function DeliveriesDialog({ webhookId, onOpenChange }: DeliveriesDialogProps) {
  const open = webhookId !== null;
  const deliveriesQ = useQuery({
    queryKey: webhookId !== null ? queryKeys.deliveries(webhookId) : ['webhooks', 'none'],
    queryFn: ({ signal }) => listDeliveries(webhookId as number, signal),
    enabled: open,
    retry: false,
  });

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content maxWidth="var(--dialog-width-md, 40rem)">
        <Dialog.Title>최근 배달</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          이 웹훅의 최근 배달 시도입니다. 배달은 at-least-once이며, 수신자는 멱등이어야 합니다.
        </Dialog.Description>
        {deliveriesQ.isLoading ? (
          <p className="text-muted">불러오는 중…</p>
        ) : deliveriesQ.isError ? (
          <Alert
            variant="page"
            message={
              deliveriesQ.error instanceof ApiError
                ? deliveriesQ.error.message
                : '배달 이력을 불러오지 못했습니다.'
            }
          />
        ) : !deliveriesQ.data || deliveriesQ.data.length === 0 ? (
          <p className="text-muted">아직 배달이 없습니다.</p>
        ) : (
          <table className="sessions-table" role="table" aria-label="배달 이력">
            <thead>
              <tr>
                <th scope="col">이벤트</th>
                <th scope="col">상태</th>
                <th scope="col">시도</th>
                <th scope="col">HTTP</th>
              </tr>
            </thead>
            <tbody>
              {deliveriesQ.data.map((d) => (
                <DeliveryRow key={d.id} delivery={d} />
              ))}
            </tbody>
          </table>
        )}
        <Flex justify="end" mt="3">
          <Dialog.Close>
            <Button type="button">닫기</Button>
          </Dialog.Close>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}

function DeliveryRow({ delivery }: { delivery: WebhookDelivery }) {
  const color =
    delivery.status === 'delivered'
      ? 'green'
      : delivery.status === 'dead'
        ? 'red'
        : ('gray' as const);
  return (
    <tr className="session-row" data-testid={`delivery-row-${delivery.id}`}>
      <td className="session-row__device">
        <code>{delivery.event}</code>
      </td>
      <td className="session-row__created">
        <Badge color={color} variant="soft" radius="full" size="1">
          {delivery.status}
        </Badge>
        {delivery.last_error ? (
          <Text as="span" size="1" color="gray" ml="2">
            {delivery.last_error}
          </Text>
        ) : null}
      </td>
      <td className="session-row__created">{delivery.attempts}</td>
      <td className="session-row__created">
        {delivery.last_status ? delivery.last_status : <span className="text-muted">—</span>}
      </td>
    </tr>
  );
}
