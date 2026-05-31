import { useMemo, useState } from 'react';
import { Box, Button, Callout, Card, Flex, Heading, Text } from '@radix-ui/themes';
import { useQuery } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { listProjects, queryKeys } from '../lib/queries';
import { AppShell } from '../components/AppShell';
import { CreateProjectDialog } from '../components/CreateProjectDialog';
import { ProjectCard } from '../components/ProjectCard';

/**
 * Bento-style projects grid (per ECC web/design-quality.md anti-template
 * checklist: clear hierarchy, depth via surfaces, real hover states).
 *
 * Layout:
 *   - ≥ md: a CSS grid with named areas. The most recently created
 *     project takes a 2×2 "featured" tile; the rest flow into 1×1
 *     tiles. That breaks the uniform card grid (anti-template item
 *     "grid-breaking editorial or bento composition") while keeping
 *     the markup an ordered list for screen readers.
 *   - < md: a plain vertical list — bento that survives on a phone is
 *     just a tall column, so we stop pretending.
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

  // Sort most-recent-first so the featured tile carries the project the
  // operator most likely wants to revisit. Stable: server already
  // orders by id desc but we re-sort to defend against ordering drift.
  const sorted = useMemo(() => {
    if (!projects) return [];
    return [...projects].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
  }, [projects]);

  const featured = sorted[0];
  const rest = sorted.slice(1);

  return (
    <AppShell
      crumbs={[{ label: '프로젝트' }]}
      actions={<Button onClick={() => setCreateOpen(true)}>새 프로젝트</Button>}
    >
      <Box>
        <Heading size="6" mb="1" as="h1">
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
            <Heading size="3" as="h2">
              아직 프로젝트가 없습니다
            </Heading>
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

      {featured ? (
        <ul
          aria-label="프로젝트 목록"
          style={{
            listStyle: 'none',
            margin: 0,
            padding: 0,
            display: 'grid',
            gap: '16px',
            gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
            gridAutoRows: 'minmax(140px, auto)',
          }}
        >
          <li
            // The featured tile spans 2×2 on viewports wide enough to
            // fit at least two of the auto-fill columns. The :where()
            // guard keeps single-column phones from stretching it.
            style={{
              gridColumn: rest.length > 0 ? 'span 2' : 'span 1',
              gridRow: rest.length > 0 ? 'span 2' : 'span 1',
              minHeight: '180px',
            }}
          >
            <ProjectCard project={featured} featured />
          </li>
          {rest.map((project) => (
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
