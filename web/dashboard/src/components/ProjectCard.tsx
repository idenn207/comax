import { Badge, Flex, Heading, Text } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';

import type { Project } from '../lib/types';

/**
 * Project card used by the bento grid on the Projects page.
 *
 * `featured=true` upgrades the visual: larger title, accent surface
 * tint, hairline accent border — used for the first (most recently
 * created) project so the eye lands on something specific rather than
 * scanning a uniform grid of identical cards.
 *
 * Motion: a 2px lift + shadow on hover/focus. Both transforms and
 * opacity stay on the compositor; no layout property animates (per ECC
 * web/coding-style.md). Reduced-motion users get the visual end-state
 * without the transition thanks to globals.css.
 *
 * Why not Radix Card? Radix Card has its own hover token system but
 * doesn't expose a hook for the lift/shadow change we want. Hand-rolling
 * a Box wrapper keeps token control end-to-end.
 */

interface ProjectCardProps {
  project: Project;
  featured?: boolean;
}

export function ProjectCard({ project, featured = false }: ProjectCardProps) {
  const created = new Date(project.created_at);
  return (
    <Link
      to="/projects/$project"
      params={{ project: project.name }}
      aria-label={`프로젝트 ${project.name} 열기`}
      style={{
        textDecoration: 'none',
        color: 'inherit',
        display: 'block',
        height: '100%',
      }}
      className="project-card"
      data-featured={featured ? 'true' : 'false'}
    >
      <article
        className="project-card-surface"
        style={{
          height: '100%',
          padding: featured ? '24px' : '16px',
          background: featured ? 'var(--color-surface-hover)' : 'var(--color-surface-elevated)',
          border: featured ? '1px solid var(--color-accent)' : '1px solid var(--color-border)',
          borderRadius: 'var(--radius-lg)',
          transition:
            'transform var(--duration-fast) var(--ease-out-expo), box-shadow var(--duration-fast) var(--ease-out-expo), border-color var(--duration-fast) var(--ease-out-expo)',
        }}
      >
        <Flex direction="column" gap={featured ? '3' : '2'} height="100%">
          <Flex align="center" justify="between" gap="2" wrap="wrap">
            {/* `as="h2"` keeps the page-level <h1>("프로젝트") as the sole
                h1 on the Projects route. Radix Heading defaults to <h1>
                otherwise, which would emit N+1 h1s per card grid. */}
            <Heading
              as="h2"
              size={featured ? '6' : '3'}
              trim="start"
              style={{ wordBreak: 'break-all' }}
            >
              {project.name}
            </Heading>
            <Badge color={featured ? 'indigo' : 'gray'} variant="soft">
              #{project.id}
            </Badge>
          </Flex>
          {featured ? (
            <Text size="2" color="gray">
              가장 최근에 만들어진 프로젝트입니다. 환경과 시크릿을 한눈에 확인하려면 이 카드를 열어
              보세요.
            </Text>
          ) : null}
          {/* inline `marginTop: auto` always wins over Radix `mt` prop, so
              the prop was effectively dead. The bottom-anchored timestamp
              works the same in featured and regular cards. */}
          <Text size="1" color="gray" style={{ marginTop: 'auto' }}>
            생성일: <time dateTime={created.toISOString()}>{created.toLocaleString('ko-KR')}</time>
          </Text>
        </Flex>
      </article>
    </Link>
  );
}
