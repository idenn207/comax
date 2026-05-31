import { useEffect, useState, type FormEvent } from 'react';
import {
  Box,
  Button,
  Callout,
  Card,
  Flex,
  Heading,
  Table,
  Text,
  TextField,
} from '@radix-ui/themes';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';

import { ApiError } from '../lib/api';
import { listAudit, queryKeys, type AuditFilter } from '../lib/queries';
import { AppShell } from '../components/AppShell';

const PAGE_LIMIT = 50;

interface AuditPageProps {
  filter: AuditFilter;
}

/**
 * Audit feed. Filters live in the URL so the operator can deep-link
 * "show me everything actor=42 did on project=alpha". Cursor-based
 * pagination (newest-first, `before=<id>`) lets us fetch additional
 * pages without page numbers — the server returns `meta.next_before`
 * when more rows exist.
 *
 * Filters are bound to a local form state; submitting the form syncs
 * them into the URL via navigate(replace=true). That decouples typing
 * from network round-trips: typing into "project" doesn't refetch on
 * every keystroke, which would otherwise spam the server.
 */
export function AuditPage({ filter }: AuditPageProps) {
  const navigate = useNavigate();
  const [projectInput, setProjectInput] = useState(filter.project ?? '');
  const [envInput, setEnvInput] = useState(filter.env ?? '');
  const [actorInput, setActorInput] = useState(filter.actor ? String(filter.actor) : '');
  const [actionInput, setActionInput] = useState(filter.action ?? '');
  const [actorError, setActorError] = useState<string | null>(null);

  // Mirror URL → form when the route prop changes (deep-link, browser
  // back/forward). Without this, an external navigate to /audit?project=X
  // would update the table but leave the inputs showing stale values.
  useEffect(() => {
    setProjectInput(filter.project ?? '');
    setEnvInput(filter.env ?? '');
    setActorInput(filter.actor ? String(filter.actor) : '');
    setActionInput(filter.action ?? '');
    setActorError(null);
  }, [filter.project, filter.env, filter.actor, filter.action]);

  const query = useInfiniteQuery({
    queryKey: queryKeys.audit({ ...filter, limit: PAGE_LIMIT }),
    initialPageParam: undefined as number | undefined,
    queryFn: ({ pageParam, signal }) =>
      listAudit({ ...filter, before: pageParam, limit: PAGE_LIMIT }, signal),
    getNextPageParam: (lastPage) => lastPage.meta.next_before,
  });

  const entries = (query.data?.pages ?? []).flatMap((p) => p.entries);
  const hasFilter = Boolean(filter.project || filter.env || filter.actor || filter.action);

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setActorError(null);
    const next: Record<string, string | number> = {};
    if (projectInput.trim()) next.project = projectInput.trim();
    if (envInput.trim()) next.env = envInput.trim();
    if (actionInput.trim()) next.action = actionInput.trim();
    if (actorInput.trim()) {
      const parsed = Number(actorInput.trim());
      if (!Number.isInteger(parsed) || parsed <= 0) {
        setActorError('actor는 양의 정수여야 합니다.');
        return;
      }
      next.actor = parsed;
    }
    void navigate({ to: '/audit', search: next, replace: true });
  }

  function onReset() {
    setProjectInput('');
    setEnvInput('');
    setActorInput('');
    setActionInput('');
    setActorError(null);
    void navigate({ to: '/audit', search: {}, replace: true });
  }

  return (
    <AppShell
      crumbs={[{ label: '감사 로그' }]}
      actions={
        <Button variant="soft" color="gray" onClick={onReset} disabled={!hasFilter}>
          필터 초기화
        </Button>
      }
    >
      <Box>
        <Heading size="6" mb="1">
          감사 로그
        </Heading>
        <Text color="gray" size="2">
          모든 변경 이벤트가 최신순으로 정렬됩니다. 필터는 URL에 저장되어 공유 가능합니다.
        </Text>
      </Box>

      <Card variant="surface" asChild>
        <form onSubmit={onSubmit} aria-label="감사 로그 필터">
          <Flex direction="column" gap="3" p="3">
            <Flex gap="3" wrap="wrap">
              <Box style={{ minWidth: 180, flex: '1 1 180px' }}>
                <Text as="label" size="1" color="gray" htmlFor="audit-project">
                  프로젝트
                </Text>
                <TextField.Root
                  id="audit-project"
                  placeholder="예: alpha"
                  value={projectInput}
                  onChange={(e) => setProjectInput(e.target.value)}
                />
              </Box>
              <Box style={{ minWidth: 180, flex: '1 1 180px' }}>
                <Text as="label" size="1" color="gray" htmlFor="audit-env">
                  환경
                </Text>
                <TextField.Root
                  id="audit-env"
                  placeholder="예: prod"
                  value={envInput}
                  onChange={(e) => setEnvInput(e.target.value)}
                />
              </Box>
              <Box style={{ minWidth: 180, flex: '1 1 180px' }}>
                <Text as="label" size="1" color="gray" htmlFor="audit-action">
                  액션
                </Text>
                <TextField.Root
                  id="audit-action"
                  placeholder="예: secret.upsert"
                  value={actionInput}
                  onChange={(e) => setActionInput(e.target.value)}
                />
              </Box>
              <Box style={{ minWidth: 180, flex: '1 1 180px' }}>
                <Text as="label" size="1" color="gray" htmlFor="audit-actor">
                  토큰 ID
                </Text>
                <TextField.Root
                  id="audit-actor"
                  placeholder="예: 3"
                  inputMode="numeric"
                  value={actorInput}
                  onChange={(e) => setActorInput(e.target.value)}
                  aria-invalid={actorError !== null}
                  aria-describedby={actorError ? 'audit-actor-error' : undefined}
                />
              </Box>
            </Flex>
            {actorError ? (
              <Text id="audit-actor-error" size="1" color="red" role="alert">
                {actorError}
              </Text>
            ) : null}
            <Flex gap="2">
              <Button type="submit">필터 적용</Button>
              <Button type="button" variant="soft" color="gray" onClick={onReset}>
                초기화
              </Button>
            </Flex>
          </Flex>
        </form>
      </Card>

      {query.error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>
            {query.error instanceof ApiError && query.error.code === 'bad_request'
              ? `필터가 올바르지 않습니다: ${query.error.message}`
              : `감사 로그를 불러오지 못했습니다.${
                  query.error instanceof ApiError ? ` (${query.error.code})` : ''
                }`}
          </Callout.Text>
        </Callout.Root>
      ) : null}

      {query.isLoading ? (
        <Text color="gray" size="2" role="status">
          불러오는 중…
        </Text>
      ) : null}

      {!query.isLoading && entries.length === 0 ? (
        <Card variant="surface">
          <Flex direction="column" gap="1" p="4" align="start">
            <Heading size="3">조회된 이벤트가 없습니다</Heading>
            <Text color="gray" size="2">
              {hasFilter
                ? '필터 조건과 일치하는 이벤트가 없습니다. 필터를 조정해 보세요.'
                : '아직 기록된 감사 이벤트가 없습니다.'}
            </Text>
          </Flex>
        </Card>
      ) : null}

      {entries.length > 0 ? (
        <Flex direction="column" gap="3">
          <Table.Root variant="surface">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeaderCell>시각</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>액션</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>대상</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>토큰</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>메타데이터</Table.ColumnHeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {entries.map((entry) => (
                <Table.Row key={entry.id}>
                  <Table.Cell>
                    <Text size="1" color="gray">
                      {new Date(entry.created_at).toLocaleString('ko-KR')}
                    </Text>
                  </Table.Cell>
                  <Table.Cell>
                    <Text
                      size="2"
                      style={{
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                      }}
                    >
                      {entry.action}
                    </Text>
                  </Table.Cell>
                  <Table.Cell>
                    <Text
                      size="2"
                      style={{
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                      }}
                    >
                      {entry.target}
                    </Text>
                  </Table.Cell>
                  <Table.Cell>
                    <Text size="2" color="gray">
                      {entry.actor_token_id ?? '—'}
                    </Text>
                  </Table.Cell>
                  <Table.Cell>
                    <Text
                      size="1"
                      color="gray"
                      style={{
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                        wordBreak: 'break-all',
                      }}
                    >
                      {entry.metadata ?? ''}
                    </Text>
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
          <Flex align="center" justify="between" gap="3">
            <Text size="1" color="gray" aria-live="polite">
              {entries.length}개 로드됨
            </Text>
            <Button
              variant="soft"
              onClick={() => void query.fetchNextPage()}
              disabled={!query.hasNextPage || query.isFetchingNextPage}
            >
              {query.isFetchingNextPage
                ? '불러오는 중…'
                : query.hasNextPage
                  ? '더 보기'
                  : '마지막 페이지'}
            </Button>
          </Flex>
        </Flex>
      ) : null}
    </AppShell>
  );
}
