import { useMemo, useState } from 'react';
import {
  Badge,
  Box,
  Button,
  Callout,
  Dialog,
  Flex,
  Heading,
  Separator,
  Text,
} from '@radix-ui/themes';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { getVersionDetail, listVersions, queryKeys, rollbackSecret } from '../lib/queries';
import type { ResolvedSecret } from '../lib/types';
import { ConfirmDialog } from './ConfirmDialog';
import { DiffViewer } from './DiffViewer';
import { useToast } from './Toast';

/**
 * Per-key version timeline + side-by-side diff viewer + rollback action.
 *
 * The plan said "side panel"; for v1 we host it inside a Dialog so we
 * don't have to thread layout state through the page. The trade-off is
 * the operator loses the table while reviewing history — acceptable for
 * the operator-of-one persona this milestone targets. A real Drawer can
 * land in M3 when the audit + diff views need to coexist.
 *
 * Version list comes from GET .../versions for the whole env (M2 read
 * endpoint) — filtered client-side to this key. The plan calls this out
 * explicitly: a per-key list endpoint can be added in M3, but until then
 * the env-level call is the canonical source of "what versions exist".
 *
 * Selection model: at most one version selected at a time. Selecting a
 * version triggers GET .../versions/{v} on demand (cached by TanStack
 * Query so re-selecting is free). Diff renders against the current
 * resolved secret.
 */
interface VersionTimelinePanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectName: string;
  envName: string;
  secret: ResolvedSecret;
}

export function VersionTimelinePanel({
  open,
  onOpenChange,
  projectName,
  envName,
  secret,
}: VersionTimelinePanelProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [selected, setSelected] = useState<number | null>(null);
  const [confirmRollbackOpen, setConfirmRollbackOpen] = useState(false);

  const versionsQuery = useQuery({
    queryKey: queryKeys.versions(projectName, envName),
    queryFn: ({ signal }) => listVersions(projectName, envName, signal),
    enabled: open,
  });

  // The env-level list returns versions for every key. We filter by
  // secret_id — the resolved view now carries it (handlers_secrets.go),
  // so two keys with the same current version number no longer collide.
  // A per-key list endpoint can replace this in M3 if the env-level
  // payload grows expensive.
  const filteredVersions = useMemo(() => {
    const all = versionsQuery.data ?? [];
    return all
      .filter((v) => v.secret_id === secret.secret_id)
      .sort((a, b) => b.version - a.version);
  }, [versionsQuery.data, secret.secret_id]);

  const detailQuery = useQuery({
    queryKey: queryKeys.versionDetail(projectName, envName, secret.key, selected ?? 0),
    queryFn: ({ signal }) => getVersionDetail(projectName, envName, secret.key, selected!, signal),
    enabled: open && selected !== null && selected !== secret.version,
  });

  const rollbackMutation = useMutation({
    mutationFn: (target: number) => rollbackSecret(projectName, envName, secret.key, target),
    onSuccess: async (updated) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify(
        'success',
        `"${updated.key}" v${updated.version} 으로 롤백 (이전 v${secret.version})`,
      );
      setSelected(null);
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      toast.notify('error', err instanceof ApiError ? err.message : '롤백에 실패했습니다.');
    },
  });

  const selectedDetail = detailQuery.data;
  const canDiff = selected !== null && selected !== secret.version;
  const canRollback = selected !== null && selected !== secret.version;

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) setSelected(null);
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="900px">
        <Dialog.Title>
          <Flex align="center" gap="2">
            <Text>{secret.key}</Text>
            <Badge color="indigo" variant="soft">
              현재 v{secret.version}
            </Badge>
          </Flex>
        </Dialog.Title>
        <Dialog.Description size="2" mb="3">
          버전을 선택하면 현재 값과의 차이를 보여주고, 그 버전의 ciphertext 로 새 버전을 만들어
          롤백할 수 있습니다.
        </Dialog.Description>

        {versionsQuery.error ? (
          <Callout.Root color="red" role="alert" mb="2">
            <Callout.Text>
              버전 이력을 불러오지 못했습니다.
              {versionsQuery.error instanceof ApiError ? ` (${versionsQuery.error.code})` : null}
            </Callout.Text>
          </Callout.Root>
        ) : null}

        <Flex gap="4" direction={{ initial: 'column', md: 'row' }} align="stretch">
          <Box style={{ minWidth: 220 }} flexGrow="0">
            <Heading size="2" mb="2">
              버전
            </Heading>
            {versionsQuery.isLoading ? (
              <Text size="2" color="gray" role="status">
                불러오는 중…
              </Text>
            ) : null}
            {filteredVersions.length === 0 && !versionsQuery.isLoading ? (
              <Text size="2" color="gray">
                이력이 없습니다.
              </Text>
            ) : null}
            <Flex direction="column" gap="1" asChild>
              <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                {filteredVersions.map((v) => {
                  const isCurrent = v.version === secret.version;
                  const isSelected = v.version === selected;
                  return (
                    <li key={v.id}>
                      <Button
                        variant={isSelected ? 'solid' : 'soft'}
                        color={isCurrent ? 'indigo' : 'gray'}
                        style={{ width: '100%', justifyContent: 'flex-start' }}
                        onClick={() => setSelected(v.version)}
                        aria-pressed={isSelected}
                        aria-label={`버전 ${v.version}${isCurrent ? ' (현재)' : ''} 선택`}
                      >
                        <Flex direction="column" align="start" gap="0">
                          <Text size="2" weight="medium">
                            v{v.version}
                            {isCurrent ? ' · 현재' : ''}
                          </Text>
                          <Text size="1" color="gray">
                            {new Date(v.created_at).toLocaleString('ko-KR')}
                          </Text>
                        </Flex>
                      </Button>
                    </li>
                  );
                })}
              </ul>
            </Flex>
          </Box>
          <Separator orientation="vertical" size="4" />
          <Box flexGrow="1">
            <Heading size="2" mb="2">
              차이
            </Heading>
            {selected === null ? (
              <Text size="2" color="gray">
                왼쪽에서 비교할 버전을 선택해 주세요.
              </Text>
            ) : null}
            {selected === secret.version ? (
              <Text size="2" color="gray">
                선택한 버전이 현재 값과 동일합니다.
              </Text>
            ) : null}
            {canDiff && detailQuery.isLoading ? (
              <Text size="2" color="gray" role="status">
                값 불러오는 중…
              </Text>
            ) : null}
            {canDiff && detailQuery.error ? (
              <Callout.Root color="red" role="alert">
                <Callout.Text>
                  버전 값을 불러오지 못했습니다.
                  {detailQuery.error instanceof ApiError ? ` (${detailQuery.error.code})` : null}
                </Callout.Text>
              </Callout.Root>
            ) : null}
            {canDiff && selectedDetail ? (
              <DiffViewer
                leftLabel={`v${selectedDetail.version}`}
                rightLabel={`v${secret.version} (현재)`}
                left={selectedDetail.value}
                right={secret.value}
              />
            ) : null}
          </Box>
        </Flex>

        <Flex gap="3" mt="4" justify="end" align="center">
          <Dialog.Close>
            <Button variant="soft" color="gray">
              닫기
            </Button>
          </Dialog.Close>
          <Button
            color="indigo"
            disabled={!canRollback || rollbackMutation.isPending}
            onClick={() => setConfirmRollbackOpen(true)}
          >
            {rollbackMutation.isPending ? '롤백 중…' : '이 버전으로 롤백'}
          </Button>
        </Flex>
      </Dialog.Content>

      {selected !== null ? (
        <ConfirmDialog
          open={confirmRollbackOpen}
          onOpenChange={setConfirmRollbackOpen}
          title={`v${selected} 로 롤백`}
          description={
            <Text size="2">
              이전 ciphertext 로 새 버전이 만들어집니다. 현재 v{secret.version} 이후 새 v
              {secret.version + 1} 가 생성되며 감사 로그에 기록됩니다.
            </Text>
          }
          confirmLabel="롤백"
          color="indigo"
          onConfirm={async () => {
            await rollbackMutation.mutateAsync(selected);
          }}
        />
      ) : null}
    </Dialog.Root>
  );
}
