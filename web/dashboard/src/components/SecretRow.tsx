import { useEffect, useRef, useState } from 'react';
import { Badge, Button, Flex, IconButton, Table, Text, TextArea } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { deleteSecret, putSecret, queryKeys } from '../lib/queries';
import type { ResolvedSecret } from '../lib/types';
import { ConfirmDialog } from './ConfirmDialog';
import { useToast } from './Toast';

/**
 * One row of the secrets table.
 *
 * State machine: viewing → editing → saving → viewing.
 *   - mask toggle exists in viewing only (editing always shows plaintext
 *     so the operator can see what they're changing).
 *   - editing mounts an autosizing textarea pre-filled with current value;
 *     Save calls PUT (creates new version), Cancel discards.
 *   - delete asks for confirmation; toast announces both outcomes.
 *
 * The "history" button opens the version timeline panel by lifting state
 * to the parent — the panel itself is a single mount per page.
 */
interface SecretRowProps {
  projectName: string;
  envName: string;
  secret: ResolvedSecret;
  onOpenHistory: (key: string) => void;
}

export function SecretRow({ projectName, envName, secret, onOpenHistory }: SecretRowProps) {
  const queryClient = useQueryClient();
  const toast = useToast();

  const [revealed, setRevealed] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(secret.value);
  const [editError, setEditError] = useState<string | null>(null);
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (editing) {
      setDraft(secret.value);
      // Microtask so the textarea is mounted before we focus it.
      window.queueMicrotask(() => textareaRef.current?.focus());
    }
  }, [editing, secret.value]);

  const saveMutation = useMutation({
    mutationFn: (value: string) => putSecret(projectName, envName, secret.key, value),
    onSuccess: async (updated) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify('success', `"${updated.key}" 저장됨 (v${updated.version})`);
      setEditing(false);
    },
    onError: (err: unknown) => {
      setEditError(err instanceof ApiError ? err.message : '저장에 실패했습니다.');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteSecret(projectName, envName, secret.key),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify('success', `"${secret.key}" 삭제됨`);
    },
    onError: (err: unknown) => {
      toast.notify('error', err instanceof ApiError ? err.message : '삭제에 실패했습니다.');
    },
  });

  function onCancel() {
    setEditing(false);
    setEditError(null);
    setDraft(secret.value);
  }

  function onSave() {
    setEditError(null);
    if (draft === secret.value) {
      // Don't issue a no-op PUT. We keep the editor open so the
      // operator can adjust the value or hit Cancel — silently
      // dismissing here would feel like the save succeeded.
      setEditError('값이 바뀌지 않았습니다. 값을 수정하거나 취소를 눌러 주세요.');
      return;
    }
    saveMutation.mutate(draft);
  }

  return (
    <>
      <Table.Row>
        <Table.RowHeaderCell>
          <Flex align="center" gap="2">
            <Text
              weight="medium"
              style={{ fontFamily: 'var(--code-font-family, ui-monospace, monospace)' }}
            >
              {secret.key}
            </Text>
            <Badge variant="soft" color="indigo">
              v{secret.version}
            </Badge>
          </Flex>
        </Table.RowHeaderCell>
        <Table.Cell>
          {editing ? (
            <Flex direction="column" gap="2">
              <TextArea
                ref={textareaRef}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                rows={Math.min(8, Math.max(2, draft.split('\n').length))}
                spellCheck={false}
                aria-label={`${secret.key} 새 값`}
              />
              {editError ? (
                <Text role="alert" color="red" size="1">
                  {editError}
                </Text>
              ) : null}
            </Flex>
          ) : (
            <Text
              style={{
                fontFamily: 'var(--code-font-family, ui-monospace, monospace)',
                wordBreak: 'break-all',
              }}
              aria-label={revealed ? `${secret.key} 값 표시됨` : `${secret.key} 값 마스킹됨`}
            >
              {revealed ? secret.value : maskValue(secret.value)}
            </Text>
          )}
        </Table.Cell>
        <Table.Cell>
          <Text size="1" color="gray">
            {new Date(secret.updated_at).toLocaleString('ko-KR')}
          </Text>
        </Table.Cell>
        <Table.Cell>
          {editing ? (
            <Flex gap="2" justify="end">
              <Button
                size="1"
                variant="soft"
                color="gray"
                onClick={onCancel}
                disabled={saveMutation.isPending}
              >
                취소
              </Button>
              <Button size="1" onClick={onSave} disabled={saveMutation.isPending}>
                {saveMutation.isPending ? '저장 중…' : '저장'}
              </Button>
            </Flex>
          ) : (
            <Flex gap="1" justify="end">
              <IconButton
                size="1"
                variant="ghost"
                color="gray"
                onClick={() => setRevealed((v) => !v)}
                aria-label={revealed ? '값 숨기기' : '값 표시'}
                title={revealed ? '값 숨기기' : '값 표시'}
              >
                {revealed ? <EyeOffIcon /> : <EyeIcon />}
              </IconButton>
              <Button
                size="1"
                variant="soft"
                color="indigo"
                onClick={() => onOpenHistory(secret.key)}
                aria-label={`${secret.key} 버전 이력`}
              >
                이력
              </Button>
              <Button size="1" variant="soft" onClick={() => setEditing(true)}>
                편집
              </Button>
              <Button
                size="1"
                variant="soft"
                color="red"
                onClick={() => setConfirmDeleteOpen(true)}
                disabled={deleteMutation.isPending}
              >
                삭제
              </Button>
            </Flex>
          )}
        </Table.Cell>
      </Table.Row>
      <ConfirmDialog
        open={confirmDeleteOpen}
        onOpenChange={setConfirmDeleteOpen}
        title={`"${secret.key}" 삭제`}
        description={
          <Text size="2">
            현재 값은 사라지지만 버전 이력은 보존됩니다. 다시 같은 키로 PUT 하면 이전 v
            {secret.version} 다음 번호로 이어집니다.
          </Text>
        }
        confirmLabel="삭제"
        onConfirm={() => deleteMutation.mutateAsync()}
      />
    </>
  );
}

function maskValue(value: string): string {
  if (value === '') return '(빈 값)';
  return '•'.repeat(Math.min(24, Math.max(8, value.length)));
}

// Inline icons keep the bundle out of @radix-ui/react-icons (not a
// direct dep here) and let the button stay focusable+aria-labeled by
// its IconButton parent. aria-hidden hides decorative SVG from SR.
function EyeIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinejoin="round"
      />
      <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}

function EyeOffIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="m3 3 18 18M10.6 6.2A10.7 10.7 0 0 1 12 6c6.5 0 10 6 10 6a18 18 0 0 1-3.1 3.8M6.6 6.6A18 18 0 0 0 2 12s3.5 6 10 6c1.6 0 3-.3 4.2-.8M9.9 9.9a3 3 0 0 0 4.2 4.2"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
