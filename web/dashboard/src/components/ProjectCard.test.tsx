import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';

import { renderWithProviders } from '../test/renderWithProviders';
import { ProjectCard } from './ProjectCard';

vi.mock('@tanstack/react-router', async () => {
  const actual =
    await vi.importActual<typeof import('@tanstack/react-router')>('@tanstack/react-router');
  return {
    ...actual,
    Link: ({
      children,
      to,
      params,
      ...rest
    }: {
      children: React.ReactNode;
      to?: string;
      params?: Record<string, string>;
    } & Record<string, unknown>) => (
      <a href={`${to}?${JSON.stringify(params)}`} {...(rest as Record<string, unknown>)}>
        {children}
      </a>
    ),
  };
});

const sampleProject = {
  id: 7,
  name: 'alpha',
  created_at: '2026-04-01T12:34:56Z',
};

describe('ProjectCard', () => {
  it('renders the name, id badge, and creation time', () => {
    renderWithProviders(<ProjectCard project={sampleProject} />);
    expect(screen.getByRole('heading', { name: 'alpha' })).toBeInTheDocument();
    expect(screen.getByText('#7')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '프로젝트 alpha 열기' })).toBeInTheDocument();
    // <time> with ISO datetime present so screen readers + a11y get a
    // machine-readable timestamp.
    expect(screen.getByText(/생성일/)).toBeInTheDocument();
    // ProjectCard formats via new Date().toISOString() which always
    // emits the millisecond field, so we normalize through the same
    // path rather than hardcoding the literal.
    expect(screen.getByRole('time')).toHaveAttribute(
      'datetime',
      new Date(sampleProject.created_at).toISOString(),
    );
  });

  it('marks the featured card with data-featured="true" and surfaces the lede', () => {
    const { container } = renderWithProviders(<ProjectCard project={sampleProject} featured />);
    const link = container.querySelector('.project-card');
    expect(link).not.toBeNull();
    expect(link).toHaveAttribute('data-featured', 'true');
    expect(screen.getByText(/가장 최근에 만들어진 프로젝트/)).toBeInTheDocument();
  });

  it('omits the featured lede on regular cards', () => {
    renderWithProviders(<ProjectCard project={sampleProject} />);
    expect(screen.queryByText(/가장 최근에 만들어진 프로젝트/)).not.toBeInTheDocument();
  });
});
