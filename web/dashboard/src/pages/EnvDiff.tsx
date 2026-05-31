import { useMemo } from 'react';
import {
  Badge,
  Box,
  Button,
  Callout,
  Card,
  Flex,
  Grid,
  Heading,
  Select,
  Text,
} from '@radix-ui/themes';
import { Link, useNavigate } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { diffEnvs, listEnvs, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';

interface EnvDiffPageProps {
  projectName: string;
  envName: string;
  against?: string;
}

/**
 * Env-vs-env diff. The path env is the LHS, `?against=<rhs>` the RHS.
 *
 * Three columns instead of a single merged list because the operator
 * almost always asks one of three questions at a time:
 *   - "what does local have that prod doesn't?" (added)
 *   - "what is in prod that local is missing?" (removed)
 *   - "what value drifted between them?" (changed)
 *
 * Click-through on a key links to the LHS secrets table for `added`
 * / `changed`, and the RHS table for `removed` — wherever the value
 * actually lives.
 */
export function EnvDiffPage({ projectName, envName, against }: EnvDiffPageProps) {
  const navigate = useNavigate();

  const envsQuery = useQuery({
    queryKey: queryKeys.envs(projectName),
    queryFn: ({ signal }) => listEnvs(projectName, signal),
  });

  const otherEnvs = useMemo(() => {
    return (envsQuery.data ?? []).filter((e) => e.name !== envName);
  }, [envsQuery.data, envName]);

  const diffEnabled = !!against && against !== envName;
  const diffQuery = useQuery({
    queryKey: queryKeys.envDiff(projectName, envName, against ?? ''),
    queryFn: ({ signal }) => diffEnvs(projectName, envName, against ?? '', signal),
    enabled: diffEnabled,
  });

  function onSelectAgainst(value: string) {
    void navigate({
      to: '/projects/$project/envs/$env/diff',
      params: { project: projectName, env: envName },
      search: { against: value },
      replace: true,
    });
  }

  const totalChanges = diffQuery.data
    ? diffQuery.data.added.length + diffQuery.data.removed.length + diffQuery.data.changed.length
    : 0;

  return (
    <AppShell
      crumbs={[
        { label: '프로젝트', to: '/' },
        { label: projectName, to: '/projects/$project', params: { project: projectName } },
        {
          label: envName,
          to: '/projects/$project/envs/$env',
          params: { project: projectName, env: envName },
        },
        { label: '비교' },
      ]}
      actions={
        <Button variant="soft" color="gray" asChild>
          <Link to="/projects/$project/envs/$env" params={{ project: projectName, env: envName }}>
            시크릿 목록
          </Link>
        </Button>
      }
    >
      <Box>
        <Heading size="6" mb="1">
          {envName} 환경 비교
        </Heading>
        <Text color="gray" size="2">
          상속이 적용된 결과를 기준으로 두 환경의 키 셋과 값을 비교합니다. 값은 표시되지 않으며 버전
          번호만 노출됩니다.
        </Text>
      </Box>

      <Flex align="center" gap="3" wrap="wrap">
        <Text size="2" color="gray">
          비교 대상
        </Text>
        <Box style={{ minWidth: 200 }}>
          <Select.Root
            value={against || undefined}
            onValueChange={onSelectAgainst}
            disabled={envsQuery.isLoading || otherEnvs.length === 0}
          >
            <Select.Trigger placeholder="다른 환경 선택" aria-label="비교할 환경" />
            <Select.Content>
              {otherEnvs.map((e) => (
                <Select.Item key={e.id} value={e.name}>
                  {e.name}
                </Select.Item>
              ))}
            </Select.Content>
          </Select.Root>
        </Box>
        {diffQuery.data ? (
          <Text size="1" color="gray" aria-live="polite">
            +{diffQuery.data.added.length} / −{diffQuery.data.removed.length} / ~
            {diffQuery.data.changed.length}
          </Text>
        ) : null}
      </Flex>

      {envsQuery.error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>환경 목록을 불러오지 못했습니다.</Callout.Text>
        </Callout.Root>
      ) : null}

      {envsQuery.isSuccess && otherEnvs.length === 0 ? (
        <Callout.Root color="gray">
          <Callout.Text>비교할 다른 환경이 없습니다. 먼저 환경을 추가하세요.</Callout.Text>
        </Callout.Root>
      ) : null}

      {!diffEnabled ? (
        <Card variant="surface">
          <Flex direction="column" gap="1" p="4" align="start">
            <Heading size="3">환경을 선택하세요</Heading>
            <Text color="gray" size="2">
              비교할 환경을 선택하면 키 셋과 변경된 값이 세 분류로 표시됩니다.
            </Text>
          </Flex>
        </Card>
      ) : null}

      {diffEnabled && diffQuery.error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>
            {diffQuery.error instanceof ApiError && diffQuery.error.code === 'not_found'
              ? '선택한 환경을 찾을 수 없습니다.'
              : diffQuery.error instanceof ApiError && diffQuery.error.code === 'bad_reference'
                ? `상속 체인에 문제가 있습니다: ${diffQuery.error.message}`
                : `차이를 계산하지 못했습니다.${
                    diffQuery.error instanceof ApiError ? ` (${diffQuery.error.code})` : ''
                  }`}
          </Callout.Text>
        </Callout.Root>
      ) : null}

      {diffEnabled && diffQuery.isLoading ? (
        <Text color="gray" size="2" role="status">
          비교 중…
        </Text>
      ) : null}

      {diffEnabled && against && diffQuery.data ? (
        <>
          {totalChanges === 0 ? (
            <Card variant="surface">
              <Flex direction="column" gap="1" p="4" align="start">
                <Heading size="3">두 환경이 동일합니다</Heading>
                <Text color="gray" size="2">
                  {envName} 환경과 {against} 환경의 키·값이 모두 일치합니다.
                </Text>
              </Flex>
            </Card>
          ) : (
            <Grid columns={{ initial: '1', md: '3' }} gap="3">
              <DiffColumn
                title={`${envName}에만 있음`}
                badge={`+${diffQuery.data.added.length}`}
                color="green"
                emptyLabel="추가된 키 없음"
                project={projectName}
                linkEnv={envName}
                items={diffQuery.data.added.map((key) => ({ key, meta: '신규 키' }))}
              />
              <DiffColumn
                title={`${against}에만 있음`}
                badge={`−${diffQuery.data.removed.length}`}
                color="red"
                emptyLabel="제거된 키 없음"
                project={projectName}
                linkEnv={against}
                items={diffQuery.data.removed.map((key) => ({
                  key,
                  meta: '비교 대상 환경에만 존재',
                }))}
              />
              <DiffColumn
                title="값이 다름"
                badge={`~${diffQuery.data.changed.length}`}
                color="amber"
                emptyLabel="값 차이 없음"
                project={projectName}
                linkEnv={envName}
                items={diffQuery.data.changed.map((c) => ({
                  key: c.key,
                  meta: `${envName} v${c.lhs_version} ↔ ${against} v${c.rhs_version}`,
                }))}
              />
            </Grid>
          )}
        </>
      ) : null}
    </AppShell>
  );
}

interface DiffItem {
  key: string;
  meta: string;
}

interface DiffColumnProps {
  title: string;
  badge: string;
  color: 'green' | 'red' | 'amber';
  emptyLabel: string;
  project: string;
  linkEnv: string;
  items: DiffItem[];
}

function DiffColumn({ title, badge, color, emptyLabel, project, linkEnv, items }: DiffColumnProps) {
  return (
    <Card variant="surface" asChild>
      <section aria-label={title}>
        <Flex direction="column" gap="2" p="3">
          <Flex align="center" justify="between">
            <Heading size="3" trim="start">
              {title}
            </Heading>
            <Badge color={color} variant="soft" aria-label={`항목 수 ${items.length}`}>
              {badge}
            </Badge>
          </Flex>
          {items.length === 0 ? (
            <Text color="gray" size="2">
              {emptyLabel}
            </Text>
          ) : (
            <Flex direction="column" gap="1">
              {items.map((item) => (
                <Link
                  key={item.key}
                  to="/projects/$project/envs/$env"
                  params={{ project, env: linkEnv }}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                  aria-label={`${item.key} — ${item.meta}`}
                >
                  <Box
                    style={{
                      padding: '8px 10px',
                      borderRadius: 'var(--radius-2)',
                      border: '1px solid var(--gray-a4)',
                      background: 'var(--gray-a2)',
                    }}
                  >
                    <Flex justify="between" align="center" gap="2">
                      <Text
                        size="2"
                        weight="medium"
                        style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                      >
                        {item.key}
                      </Text>
                      <Text size="1" color="gray">
                        {item.meta}
                      </Text>
                    </Flex>
                  </Box>
                </Link>
              ))}
            </Flex>
          )}
        </Flex>
      </section>
    </Card>
  );
}
