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
import { PageHeader } from '../components/PageHeader';

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

  // Eyebrow promotes the missing-key count first per Design Principle #3
  // ("누락은 1급 시각 신호"). The added count never makes the eyebrow —
  // new keys are informational, not an alarm. Order: vs target → 누락 →
  // 변경 → (일치 when all three buckets are empty).
  const eyebrow = (() => {
    if (!against) return '비교 대상 선택';
    if (!diffQuery.data) return `vs ${against}`;
    const { added, removed, changed } = diffQuery.data;
    const parts: string[] = [`vs ${against}`];
    if (removed.length > 0) parts.push(`누락 ${removed.length}`);
    if (changed.length > 0) parts.push(`변경 ${changed.length}`);
    if (removed.length === 0 && changed.length === 0 && added.length === 0) parts.push('일치');
    return parts.join(' · ');
  })();

  return (
    <AppShell
      active="projects"
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
    >
      <PageHeader
        title={`${envName} 환경 비교`}
        eyebrow={eyebrow}
        actions={
          <Button variant="soft" color="gray" asChild>
            <Link to="/projects/$project/envs/$env" params={{ project: projectName, env: envName }}>
              시크릿 목록
            </Link>
          </Button>
        }
      />

      <Flex align="center" gap="3" wrap="wrap">
        <Text size="2" color="gray">
          비교 대상
        </Text>
        <Box className="min-w-[200px]">
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
          <Flex direction="column" gap="2" p="5" align="start">
            <Heading as="h2" size="4" trim="start">
              비교할 환경을 선택하세요
            </Heading>
            <Text size="2" color="gray">
              위 ‘비교 대상’ 드롭다운에서 다른 환경을 골라 누락·변경·신규 키를 확인합니다.
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
              <Flex direction="column" gap="2" p="5" align="start">
                <Heading as="h2" size="4" trim="start">
                  두 환경이 동일합니다
                </Heading>
                <Text color="gray" size="2">
                  {envName} ↔ {against}: 키와 값이 모두 일치.
                </Text>
              </Flex>
            </Card>
          ) : (
            /* Column reading order is the signal: removed (emphasized)
               first because Design Principle #3 names "누락" a 1급 시각
               signal, then changed (drift), then added (informational).
               Symmetric 3-col grid said "all three are equal news"; this
               order makes the operator's eye land on what they must act
               on. The redundant per-row meta ("신규 키" / "비교 대상
               환경에만 존재") is dropped from added/removed — those
               labels were synonyms of the column title and stole weight
               from the key name itself. */
            <Grid columns={{ initial: '1', md: '3' }} gap="3">
              <DiffColumn
                title={`${against}에만 있음`}
                badge={`−${diffQuery.data.removed.length}`}
                color="red"
                emptyLabel="제거된 키 없음"
                project={projectName}
                linkEnv={against}
                emphasized
                items={diffQuery.data.removed.map((key) => ({ key }))}
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
              <DiffColumn
                title={`${envName}에만 있음`}
                badge={`+${diffQuery.data.added.length}`}
                color="green"
                emptyLabel="추가된 키 없음"
                project={projectName}
                linkEnv={envName}
                items={diffQuery.data.added.map((key) => ({ key }))}
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
  /** Optional per-row metadata. Only `changed` rows carry it (version pair);
   *  removed/added rows omit it so the key name itself reads as the signal. */
  meta?: string;
}

interface DiffColumnProps {
  title: string;
  badge: string;
  color: 'green' | 'red' | 'amber';
  emptyLabel: string;
  project: string;
  linkEnv: string;
  items: DiffItem[];
  /** When true, the column tilts toward danger-soft surface + larger title.
   *  Reserved for the "removed = missing" column per Design Principle #3. */
  emphasized?: boolean;
}

function DiffColumn({
  title,
  badge,
  color,
  emptyLabel,
  project,
  linkEnv,
  items,
  emphasized = false,
}: DiffColumnProps) {
  // Globals.css owns the surface + title scale. Radix Card asChild paints
  // its own background, which fights the `.diff-col-emphasized` ground
  // tint; raw <section> with the globals utility class wins.
  const className = emphasized ? 'diff-col diff-col-emphasized' : 'diff-col';
  return (
    <section aria-label={title} className={className}>
      <Flex align="center" justify="between" gap="2">
        <h2 className="diff-col-title">{title}</h2>
        <Badge color={color} variant="soft" aria-label={`항목 수 ${items.length}`}>
          {badge}
        </Badge>
      </Flex>
      {items.length === 0 ? (
        <span className="diff-col-empty">{emptyLabel}</span>
      ) : (
        <Flex direction="column" gap="1">
          {items.map((item) => (
            <Link
              key={item.key}
              to="/projects/$project/envs/$env"
              params={{ project, env: linkEnv }}
              className="diff-key"
              aria-label={item.meta ? `${item.key} — ${item.meta}` : item.key}
            >
              <div className="flex justify-between items-center gap-2">
                <Text size="2" weight="medium" className="mono">
                  {item.key}
                </Text>
                {item.meta ? (
                  <Text size="1" color="gray">
                    {item.meta}
                  </Text>
                ) : null}
              </div>
            </Link>
          ))}
        </Flex>
      )}
    </section>
  );
}
