import { useState } from 'react';
import { Button, Callout, Card, Flex, Grid, Text } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { listEnvs, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { CreateEnvDialog } from '../components/CreateEnvDialog';
import { PageHeader } from '../components/PageHeader';

interface ProjectPageProps {
  projectName: string;
}

export function ProjectPage({ projectName }: ProjectPageProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const {
    data: envs,
    isLoading,
    error,
    refetch,
    isRefetching,
  } = useQuery({
    queryKey: queryKeys.envs(projectName),
    queryFn: ({ signal }) => listEnvs(projectName, signal),
  });

  return (
    <AppShell active="projects" crumbs={[{ label: '프로젝트', to: '/' }, { label: projectName }]}>
      <PageHeader
        title={projectName}
        eyebrow={envs && envs.length > 0 ? `${envs.length}개 환경` : undefined}
        actions={
          <Button onClick={() => setCreateOpen(true)} disabled={isLoading}>
            새 환경
          </Button>
        }
      />

      {error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>
            {error instanceof ApiError && error.code === 'not_found'
              ? '프로젝트를 찾을 수 없습니다. 이름이 정확한지 확인해 주세요.'
              : `환경 목록을 불러오지 못했습니다.${
                  error instanceof ApiError ? ` (${error.code})` : ''
                }`}
          </Callout.Text>
          <Flex mt="2" gap="2">
            <Button size="1" variant="soft" onClick={() => void refetch()} disabled={isRefetching}>
              {isRefetching ? '재시도 중…' : '재시도'}
            </Button>
            <Button size="1" variant="soft" color="gray" asChild>
              <Link to="/">프로젝트 목록으로</Link>
            </Button>
          </Flex>
        </Callout.Root>
      ) : null}

      {isLoading ? (
        <Text color="gray" size="2" role="status">
          불러오는 중…
        </Text>
      ) : null}

      {envs && envs.length === 0 ? (
        <Card variant="surface">
          <Flex direction="column" gap="3" p="5" align="start">
            <h2 className="text-lg font-semibold tracking-tight">아직 환경이 없습니다</h2>
            <Button mt="1" onClick={() => setCreateOpen(true)}>
              첫 환경 만들기
            </Button>
          </Flex>
        </Card>
      ) : null}

      {envs && envs.length > 0 ? (
        <Grid columns={{ initial: '1', sm: '2', md: '3' }} gap="3">
          {envs.map((env) => (
            <Link
              key={env.id}
              to="/projects/$project/envs/$env"
              params={{ project: projectName, env: env.name }}
              className="env-tile"
              aria-label={`환경 ${env.name} 열기`}
            >
              <div className="flex items-center justify-between gap-2">
                <h2 className="mono text-md font-semibold tracking-tight m-0">{env.name}</h2>
                {env.inherits_from ? (
                  <span
                    className="chip chip-mono"
                    title={`상속: ${env.inherits_from}`}
                    aria-label={`${env.inherits_from} 상속`}
                  >
                    ← {env.inherits_from}
                  </span>
                ) : null}
              </div>
              <Text size="1" className="text-muted">
                생성일: {new Date(env.created_at).toLocaleString('ko-KR')}
              </Text>
            </Link>
          ))}
        </Grid>
      ) : null}

      <CreateEnvDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        projectName={projectName}
        existingEnvs={envs ?? []}
      />
    </AppShell>
  );
}
