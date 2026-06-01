import { useMemo, useState } from 'react';
import { Button, Callout, Card, Flex, Text } from '@radix-ui/themes';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { listProjects, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { CreateProjectDialog } from '../components/CreateProjectDialog';
import { PageHeader } from '../components/PageHeader';
import { ProjectCard } from '../components/ProjectCard';

/**
 * Projects grid (V1 — Doppler-literal monochrome).
 *
 * Bento was retired in the 2026-06-01 live distill session: every card
 * is the same size, sorted by creation time (newest first). The single-
 * column collapse on narrow viewports is handled by the auto-fill grid
 * itself rather than a media-query branch — `minmax(320px, 1fr)` resolves
 * to one column the moment the container drops below ~336px.
 *
 * No descriptive subtitle: the operator already knows what a project is
 * (the heading and crumb say it), and the empty state teaches the model
 * for first-run users.
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

  const sorted = useMemo(() => {
    if (!projects) return [];
    return [...projects].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
  }, [projects]);

  const total = projects?.length ?? 0;

  return (
    <AppShell active="projects" crumbs={[{ label: '프로젝트' }]}>
      <PageHeader
        title="프로젝트"
        eyebrow={total > 0 ? `${total}개` : undefined}
        actions={<Button onClick={() => setCreateOpen(true)}>새 프로젝트</Button>}
      />

      {error ? (
        <Callout.Root color="red" role="alert" mb="4">
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
          <Flex direction="column" gap="3" p="5" align="start">
            <h2 className="text-lg font-semibold tracking-tight">아직 프로젝트가 없습니다</h2>
            <Text color="gray" size="2">
              첫 프로젝트를 만들면 환경과 시크릿이 이곳에 표시됩니다.
            </Text>
            <Button mt="1" onClick={() => setCreateOpen(true)}>
              첫 프로젝트 만들기
            </Button>
          </Flex>
        </Card>
      ) : null}

      {sorted.length > 0 ? (
        <ul aria-label="프로젝트 목록" className="projects-grid">
          {sorted.map((project) => (
            <li key={project.id}>
              <ProjectCard project={project} />
            </li>
          ))}
        </ul>
      ) : null}

      <CreateProjectDialog open={createOpen} onOpenChange={setCreateOpen} />
    </AppShell>
  );
}
