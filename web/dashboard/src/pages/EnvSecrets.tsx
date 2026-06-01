import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Callout, Card, Flex, Table, Text, TextField } from '@radix-ui/themes';
import { Link, useRouterState } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { formatDotenv } from '../lib/dotenv';
import { listSecrets, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { AddSecretDialog } from '../components/AddSecretDialog';
import { PageHeader } from '../components/PageHeader';
import { SecretRow } from '../components/SecretRow';
import { VersionTimelinePanel } from '../components/VersionTimelinePanel';
import { useToast } from '../components/Toast';

type SortKey = 'key' | 'updated_at';

interface EnvSecretsPageProps {
  projectName: string;
  envName: string;
}

export function EnvSecretsPage({ projectName, envName }: EnvSecretsPageProps) {
  const toast = useToast();
  const [filter, setFilter] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('key');
  const [addOpen, setAddOpen] = useState(false);
  const [historyKey, setHistoryKey] = useState<string | null>(null);

  const {
    data: secrets,
    isLoading,
    error,
    refetch,
    isRefetching,
  } = useQuery({
    queryKey: queryKeys.secrets(projectName, envName),
    queryFn: ({ signal }) => listSecrets(projectName, envName, signal),
  });

  const existingKeys = useMemo(() => {
    return new Set((secrets ?? []).map((s) => s.key));
  }, [secrets]);

  const visible = useMemo(() => {
    const trimmed = filter.trim().toLowerCase();
    const rows = (secrets ?? []).filter((s) =>
      trimmed === '' ? true : s.key.toLowerCase().includes(trimmed),
    );
    const sorted = [...rows].sort((a, b) => {
      if (sortKey === 'key') return a.key.localeCompare(b.key);
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    });
    return sorted;
  }, [secrets, filter, sortKey]);

  const historySecret = useMemo(() => {
    if (!historyKey) return null;
    return (secrets ?? []).find((s) => s.key === historyKey) ?? null;
  }, [historyKey, secrets]);

  // Deep-link from the command palette: navigating to
  // /projects/$p/envs/$e#$key lands here with a hash. Once the secrets
  // list is in the DOM we scroll the matching row into view and pulse
  // an ink highlight that fades over 1.5s so the operator's eye lands
  // on the right line. Read the hash through router state (not via a
  // `hashchange` listener) because TanStack Router uses pushState,
  // which does not fire hashchange — picking a second key while
  // already on this env would otherwise be silently missed.
  const routerHash = useRouterState({ select: (s) => s.location.hash });
  const hash = typeof routerHash === 'string' ? routerHash.replace(/^#/, '') : '';
  useEffect(() => {
    if (!hash || !secrets || secrets.length === 0) return;
    let decoded: string;
    try {
      decoded = decodeURIComponent(hash);
    } catch {
      decoded = hash;
    }
    const escaped = typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(decoded) : decoded;
    const frame = window.requestAnimationFrame(() => {
      const row = document.querySelector<HTMLElement>(`[data-secret-key="${escaped}"]`);
      if (!row) return;
      row.scrollIntoView({ block: 'center', behavior: 'smooth' });
      row.classList.add('cmdk-row-highlight');
      window.setTimeout(() => {
        row.classList.remove('cmdk-row-highlight');
      }, 1500);
    });
    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [hash, secrets]);

  async function onCopyAll() {
    const rows = visible.map((s) => ({ key: s.key, value: s.value }));
    if (rows.length === 0) {
      toast.notify('error', '복사할 시크릿이 없습니다.');
      return;
    }
    const body = formatDotenv(rows);
    try {
      await navigator.clipboard.writeText(body);
      toast.notify('success', `${rows.length}개 시크릿을 .env 형식으로 클립보드에 복사했습니다.`);
    } catch {
      toast.notify('error', '클립보드 접근에 실패했습니다.');
    }
  }

  return (
    <AppShell
      active="projects"
      crumbs={[
        { label: '프로젝트', to: '/' },
        { label: projectName, to: '/projects/$project', params: { project: projectName } },
        { label: envName },
      ]}
    >
      <PageHeader
        title={envName}
        eyebrow={`${projectName} / 환경`}
        actions={
          <>
            <Button variant="soft" color="gray" asChild>
              <Link
                to="/projects/$project/envs/$env/diff"
                params={{ project: projectName, env: envName }}
                search={{}}
                aria-label="다른 환경과 비교"
              >
                환경 비교
              </Link>
            </Button>
            <Button
              variant="soft"
              color="gray"
              onClick={onCopyAll}
              disabled={isLoading || visible.length === 0}
              aria-label=".env 형식으로 복사"
            >
              .env 복사
            </Button>
            <Button onClick={() => setAddOpen(true)} disabled={isLoading}>
              새 시크릿
            </Button>
          </>
        }
      />

      {error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>
            {error instanceof ApiError && error.code === 'not_found'
              ? '환경을 찾을 수 없습니다.'
              : error instanceof ApiError && error.code === 'bad_reference'
                ? `상속 체인에 문제가 있습니다: ${error.message}`
                : `시크릿 목록을 불러오지 못했습니다.${
                    error instanceof ApiError ? ` (${error.code})` : ''
                  }`}
          </Callout.Text>
          <Flex mt="2" gap="2">
            <Button size="1" variant="soft" onClick={() => void refetch()} disabled={isRefetching}>
              {isRefetching ? '재시도 중…' : '재시도'}
            </Button>
            <Button size="1" variant="soft" color="gray" asChild>
              <Link to="/projects/$project" params={{ project: projectName }}>
                환경 목록으로
              </Link>
            </Button>
          </Flex>
        </Callout.Root>
      ) : null}

      {isLoading ? (
        <Text color="gray" size="2" role="status">
          불러오는 중…
        </Text>
      ) : null}

      {secrets && secrets.length === 0 ? (
        <Card variant="surface">
          <Flex direction="column" gap="3" p="5" align="start">
            <h2 className="text-lg font-semibold tracking-tight m-0">시크릿이 없습니다</h2>
            <Button mt="1" onClick={() => setAddOpen(true)}>
              첫 시크릿 추가
            </Button>
          </Flex>
        </Card>
      ) : null}

      {secrets && secrets.length > 0 ? (
        <Flex direction="column" gap="3">
          <Flex gap="3" align="center" wrap="wrap">
            <Box className="min-w-[200px] flex-[1_1_240px]">
              <TextField.Root
                placeholder="키 검색"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                aria-label="키 이름으로 필터"
              />
            </Box>
            <Flex gap="1" align="center">
              <Text size="2" color="gray">
                정렬:
              </Text>
              <Button
                size="1"
                variant={sortKey === 'key' ? 'solid' : 'soft'}
                color="gray"
                onClick={() => setSortKey('key')}
                aria-pressed={sortKey === 'key'}
              >
                키
              </Button>
              <Button
                size="1"
                variant={sortKey === 'updated_at' ? 'solid' : 'soft'}
                color="gray"
                onClick={() => setSortKey('updated_at')}
                aria-pressed={sortKey === 'updated_at'}
              >
                최근 변경
              </Button>
            </Flex>
            <Text size="1" color="gray" ml="auto" aria-live="polite">
              {visible.length} / {secrets.length}
            </Text>
          </Flex>

          <Table.Root variant="surface">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeaderCell>키</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>값</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell>마지막 변경</Table.ColumnHeaderCell>
                <Table.ColumnHeaderCell width="280px">
                  <span className="visually-hidden">작업</span>
                </Table.ColumnHeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {visible.map((secret) => (
                <SecretRow
                  key={`${secret.key}-${secret.version}`}
                  projectName={projectName}
                  envName={envName}
                  secret={secret}
                  onOpenHistory={(key) => setHistoryKey(key)}
                />
              ))}
              {visible.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan={4}>
                    <Text color="gray" size="2">
                      필터에 해당하는 키가 없습니다.
                    </Text>
                  </Table.Cell>
                </Table.Row>
              ) : null}
            </Table.Body>
          </Table.Root>
        </Flex>
      ) : null}

      <AddSecretDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        projectName={projectName}
        envName={envName}
        existingKeys={existingKeys}
      />

      {historySecret ? (
        <VersionTimelinePanel
          open={historyKey !== null}
          onOpenChange={(next) => setHistoryKey(next ? historySecret.key : null)}
          projectName={projectName}
          envName={envName}
          secret={historySecret}
        />
      ) : null}
    </AppShell>
  );
}
