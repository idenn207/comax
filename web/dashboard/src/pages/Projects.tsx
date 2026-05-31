import { useState } from 'react';
import { Badge, Box, Button, Callout, Card, Flex, Grid, Heading, Text } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { listProjects, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { CreateProjectDialog } from '../components/CreateProjectDialog';

/**
 * Bento-style projects grid (per ECC web/design-quality.md anti-template
 * checklist: clear hierarchy, depth via surfaces, real hover states).
 *
 * Empty state is the first surface a fresh server shows — we keep it
 * intentional rather than blank to avoid the "default template" feel.
 */
export function ProjectsPage() {
  const [createOpen, setCreateOpen] = useState(false);
  const {
    data: projects,
    isLoading,
    error,
    refetch,
    isRefetching,
  } = useQuery({
    queryKey: queryKeys.projects(),
    queryFn: ({ signal }) => listProjects(signal),
  });

  return (
    <AppShell
      crumbs={[{ label: '프로젝트' }]}
      actions={<Button onClick={() => setCreateOpen(true)}>새 프로젝트</Button>}
    >
      <Box>
        <Heading size="6" mb="1">
          프로젝트
        </Heading>
        <Text color="gray" size="2">
          시크릿은 프로젝트 → 환경 → 키 순으로 분류됩니다. 프로젝트를 선택하면 환경 목록으로
          이동합니다.
        </Text>
      </Box>

      {error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Text>
            프로젝트 목록을 불러오지 못했습니다.
            {error instanceof ApiError ? ` (${error.code})` : null}
          </Callout.Text>
          <Flex mt="2">
            <Button size="1" variant="soft" onClick={() => void refetch()} disabled={isRefetching}>
              {isRefetching ? '재시도 중…' : '재시도'}
            </Button>
          </Flex>
        </Callout.Root>
      ) : null}

      {isLoading ? (
        <Text color="gray" size="2" role="status">
          불러오는 중…
        </Text>
      ) : null}

      {projects && projects.length === 0 ? (
        <Card variant="surface">
          <Flex direction="column" gap="2" p="4" align="start">
            <Heading size="3">아직 프로젝트가 없습니다</Heading>
            <Text color="gray" size="2">
              첫 프로젝트를 만들고 환경/시크릿을 추가하면 CLI나 다른 서비스에서 곧바로 사용할 수
              있습니다.
            </Text>
            <Button mt="2" onClick={() => setCreateOpen(true)}>
              첫 프로젝트 만들기
            </Button>
          </Flex>
        </Card>
      ) : null}

      {projects && projects.length > 0 ? (
        <Grid columns={{ initial: '1', sm: '2', md: '3' }} gap="3">
          {projects.map((project) => (
            <Link
              key={project.id}
              to="/projects/$project"
              params={{ project: project.name }}
              style={{ textDecoration: 'none', color: 'inherit' }}
              aria-label={`프로젝트 ${project.name} 열기`}
            >
              <Card variant="surface" asChild>
                <article>
                  <Flex direction="column" gap="2" p="3">
                    <Flex align="center" justify="between">
                      <Heading size="3" trim="start">
                        {project.name}
                      </Heading>
                      <Badge color="indigo" variant="soft">
                        #{project.id}
                      </Badge>
                    </Flex>
                    <Text size="1" color="gray">
                      생성일: {new Date(project.created_at).toLocaleString('ko-KR')}
                    </Text>
                  </Flex>
                </article>
              </Card>
            </Link>
          ))}
        </Grid>
      ) : null}

      <CreateProjectDialog open={createOpen} onOpenChange={setCreateOpen} />
    </AppShell>
  );
}
