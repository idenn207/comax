import { useState, type FormEvent } from 'react';
import { Button, Dialog, Flex, TextField } from '@radix-ui/themes';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Alert } from '../components/Alert';
import { AppShell } from '../components/AppShell';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { FormField } from '../components/FormField';
import { PageHeader } from '../components/PageHeader';
import { TokenRow } from '../components/TokenRow';
import { ApiError } from '../lib/api';
import { createToken, listTokens, queryKeys, revokeToken } from '../lib/queries';
import type { CreatedToken } from '../lib/types';
import { NAME_FORMAT_HINT, nameError } from '../lib/validate';

/**
 * Tokens page — `/settings/tokens`.
 *
 * Service-token management: issue (non-admin), list, revoke. The server
 * requires an admin token; a non-admin session gets 403 on GET /tokens,
 * which we surface as an "admin only" notice rather than an error banner
 * (정직함 — the operator learns why, not that something broke).
 *
 * Design decisions (DESIGN.md):
 *   - Table reuses .sessions-table, mirroring the Sessions page vocabulary.
 *   - Issue flow shows the plaintext exactly once in a two-phase dialog —
 *     the value cannot be re-fetched, so the dialog warns before it closes.
 *   - Revoke carries the honest "회수 이전 read 는 되돌릴 수 없다" caveat.
 */
export function TokensPage() {
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const tokensQ = useQuery({
    queryKey: queryKeys.tokens(),
    queryFn: ({ signal }) => listTokens(signal),
    // Admin-only endpoint: a 403 is an expected outcome for a non-admin
    // session, not a transient failure, so don't retry it.
    retry: false,
  });

  const revoke = useMutation({
    mutationFn: (id: number) => revokeToken(id),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.tokens() });
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : '토큰을 회수하지 못했습니다.');
    },
  });

  const forbidden =
    tokensQ.isError && tokensQ.error instanceof ApiError && tokensQ.error.code === 'forbidden';

  const pendingToken =
    pendingId !== null ? tokensQ.data?.find((t) => t.id === pendingId) ?? null : null;

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
    <AppShell
      active="tokens"
      crumbs={[{ label: '설정', to: '/' }, { label: '토큰' }]}
    >
      <PageHeader
        title="서비스 토큰"
        actions={
          forbidden ? undefined : (
            <Button type="button" onClick={() => setCreateOpen(true)}>
              새 토큰
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
          토큰 관리는 관리자 토큰으로만 가능합니다. 관리자에게 발급을 요청하세요.
        </p>
      ) : tokensQ.isError ? (
        <Alert
          variant="page"
          message={
            tokensQ.error instanceof ApiError
              ? tokensQ.error.message
              : '토큰 목록을 불러오지 못했습니다.'
          }
        />
      ) : tokensQ.isLoading ? (
        <p className="text-muted">불러오는 중…</p>
      ) : !tokensQ.data || tokensQ.data.length === 0 ? (
        <p className="text-muted">발급된 토큰이 없습니다.</p>
      ) : (
        <table className="sessions-table" role="table" aria-label="서비스 토큰 목록">
          <thead>
            <tr>
              <th scope="col">이름</th>
              <th scope="col">생성</th>
              <th scope="col">마지막 사용</th>
              <th scope="col" className="sr-only">
                액션
              </th>
            </tr>
          </thead>
          <tbody>
            {tokensQ.data.map((t) => (
              <TokenRow
                key={t.id}
                token={t}
                onRevoke={startRevoke}
                busy={revoke.isPending && revoke.variables === t.id}
              />
            ))}
          </tbody>
        </table>
      )}

      <CreateTokenDialog open={createOpen} onOpenChange={setCreateOpen} />

      <ConfirmDialog
        open={pendingId !== null}
        onOpenChange={(open) => {
          if (!open) setPendingId(null);
        }}
        title="토큰 회수"
        description={
          <div className="confirm-dialog__body">
            <p>
              <strong>{pendingToken ? pendingToken.name : '이 토큰'}</strong>은 즉시 인증에 사용할
              수 없게 됩니다 (bearer·대시보드 세션 양쪽).
            </p>
            <p className="confirm-dialog__caveat">
              다만 회수 이전에 이미 read 된 시크릿 값은 되돌릴 수 없습니다. 유출이 의심된다면 값
              자체도 로테이션하세요.
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

interface CreateTokenDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Two-phase issue dialog. Phase 1 collects the name; phase 2 shows the
 * plaintext exactly once (with a copy button + a warning that it cannot be
 * re-fetched). Closing the dialog resets both phases.
 */
function CreateTokenDialog({ open, onOpenChange }: CreateTokenDialogProps) {
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [nameFieldError, setNameFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [created, setCreated] = useState<CreatedToken | null>(null);
  const [copied, setCopied] = useState(false);

  const mutation = useMutation({
    mutationFn: (value: string) => createToken(value),
    onSuccess: async (token) => {
      await qc.invalidateQueries({ queryKey: queryKeys.tokens() });
      setCreated(token);
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.code === 'conflict') {
          setFormError('같은 이름의 토큰이 이미 존재합니다.');
          return;
        }
        if (err.code === 'forbidden') {
          setFormError('토큰 발급은 관리자만 가능합니다.');
          return;
        }
        setFormError(err.message);
        return;
      }
      setFormError('알 수 없는 오류로 발급에 실패했습니다. 잠시 후 다시 시도해 주세요.');
    },
  });

  function reset() {
    setName('');
    setNameFieldError(null);
    setFormError(null);
    setCreated(null);
    setCopied(false);
    mutation.reset();
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setNameFieldError(null);
    setFormError(null);
    const trimmed = name.trim();
    const validation = nameError('token name', trimmed);
    if (validation) {
      setNameFieldError(validation);
      return;
    }
    mutation.mutate(trimmed);
  }

  async function copyToken() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.token);
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
            <Dialog.Title>토큰 발급됨</Dialog.Title>
            <Dialog.Description size="2" mb="3">
              아래 토큰은 <strong>지금 한 번만</strong> 표시됩니다. 복사해서 GitHub 저장소의 secret
              (예: <code>COMAX_TOKEN</code>)에 저장하세요. 닫으면 다시 볼 수 없습니다.
            </Dialog.Description>
            <Flex direction="column" gap="3">
              <TextField.Root
                value={created.token}
                readOnly
                aria-label="발급된 토큰"
                onFocus={(e) => e.currentTarget.select()}
              />
              <Flex gap="3" justify="end">
                <Button type="button" variant="soft" onClick={copyToken}>
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
            <Dialog.Title>새 서비스 토큰</Dialog.Title>
            <Dialog.Description size="2" mb="3">
              CI 러너에서 쓸 non-admin 토큰을 발급합니다. 발급된 토큰은 다른 토큰을 만들거나 회수할
              수 없습니다.
            </Dialog.Description>
            <form onSubmit={onSubmit}>
              <Flex direction="column" gap="3">
                <FormField
                  id="create-token-name"
                  label="토큰 이름"
                  hint={NAME_FORMAT_HINT}
                  error={nameFieldError}
                >
                  {(fieldProps) => (
                    <TextField.Root
                      {...fieldProps}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="예: ci-github"
                      autoFocus
                      spellCheck={false}
                    />
                  )}
                </FormField>
                <Alert variant="form" message={formError} />
                <Flex gap="3" justify="end">
                  <Dialog.Close>
                    <Button variant="soft" color="gray" type="button">
                      취소
                    </Button>
                  </Dialog.Close>
                  <Button type="submit" disabled={mutation.isPending || name.trim() === ''}>
                    {mutation.isPending ? '발급 중…' : '발급'}
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
