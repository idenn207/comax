import { useState } from 'react';
import { Badge, Box, Button, Callout, Card, Flex, Grid, Heading, Text } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { listEnvs, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { CreateEnvDialog } from '../components/CreateEnvDialog';

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
    <AppShell
      crumbs={[{ label: '프로젝트', to: '/' }, { label: projectName }]}
      actions={
        <Button onClick={() => setCreateOpen(true)} disabled={isLoading}>
          새 환경
        </Button>
      }
    >
      <Box>
        <Heading size="6" mb="1">
          {projectName}
        </Heading>
        <Text color="gray" size="2">
          환경은 시크릿이 실제로 저장되는 그릇입니다. 다른 환경을 상속하면 키 단위 오버라이드만
          남깁니다.
        </Text>
      </Box>

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
          <Flex direction="column" gap="2" p="4" align="start">
            <Heading size="3">아직 환경이 없습니다</Heading>
            <Text color="gray" size="2">
              base, local, prod 같은 환경을 만들고 시크릿을 넣으면 CLI/SDK에서 곧장 사용할 수
              있습니다.
            </Text>
            <Button mt="2" onClick={() => setCreateOpen(true)}>
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
              style={{ textDecoration: 'none', color: 'inherit' }}
              aria-label={`환경 ${env.name} 열기`}
            >
              <Card variant="surface" asChild>
                <article>
                  <Flex direction="column" gap="2" p="3">
                    <Flex align="center" justify="between">
                      <Heading size="3" trim="start">
                        {env.name}
                      </Heading>
                      {env.inherits_from ? (
                        <Badge color="indigo" variant="soft" title={`상속: ${env.inherits_from}`}>
                          ← {env.inherits_from}
                        </Badge>
                      ) : null}
                    </Flex>
                    <Text size="1" color="gray">
                      생성일: {new Date(env.created_at).toLocaleString('ko-KR')}
                    </Text>
                  </Flex>
                </article>
              </Card>
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
