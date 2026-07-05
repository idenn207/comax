import { Link } from '@tanstack/react-router';

import type { Project } from '../lib/types';

/**
 * Project card used by the Projects grid.
 *
 * V1 (Doppler-literal monochrome) was accepted in the 2026-06-01 live
 * distill session: every card is the same size — no bento, no featured
 * tile, no descriptive lede, no creation timestamp, no #id chip. The
 * cells the operator scans for are the project name and how many envs
 * sit under it. Both ship in this card; everything else is reachable
 * from the page that this card links to.
 *
 * The `N configs` chip uses Doppler's vocabulary on purpose: in this
 * dashboard "config" === environment (dev / staging / prod). Bottom-left
 * alignment is intentional — the count is reference data, not the
 * affordance, and parking it at the bottom lets the title carry the
 * scan path.
 *
 * Hover: border-color shifts to border-strong. No translateY / shadow
 * lift; DESIGN.md "조용함" rules out animating the surface up on hover
 * when a colour change reads cleaner at the operator's normal viewing
 * distance. Reduced-motion users get the same end state instantly.
 */

interface ProjectCardProps {
  project: Project;
}

export function ProjectCard({ project }: ProjectCardProps) {
  const envCount = project.env_count;
  return (
    <Link
      to="/projects/$project"
      params={{ project: project.name }}
      aria-label={`프로젝트 ${project.name} 열기`}
      className="project-card"
    >
      <article className="project-card-surface">
        <h2 className="project-card-title" title={project.name}>
          {project.name}
        </h2>
        <span
          className="chip project-card-chip"
          aria-label={`환경 ${envCount}개`}
        >
          {envCount} configs
        </span>
      </article>
    </Link>
  );
}
