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
  env_count: 3,
};

describe('ProjectCard', () => {
  it('renders the name as the only heading and the configs chip with env_count', () => {
    renderWithProviders(<ProjectCard project={sampleProject} />);
    expect(screen.getByRole('heading', { name: 'alpha' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '프로젝트 alpha 열기' })).toBeInTheDocument();
    // V1 spec carries the env count as Doppler vocabulary; the chip's
    // aria-label localizes it for Korean screen readers.
    expect(screen.getByText('3 configs')).toBeInTheDocument();
    expect(screen.getByLabelText('환경 3개')).toBeInTheDocument();
  });

  it('omits the legacy #id chip, creation timestamp, and featured flag', () => {
    // V1 stripped the bento featured tile and the id/created_at metadata.
    // These assertions pin the contract so a future re-introduction of
    // either signal trips a test instead of silently slipping in.
    const { container } = renderWithProviders(<ProjectCard project={sampleProject} />);
    expect(screen.queryByText('#7')).not.toBeInTheDocument();
    expect(screen.queryByText(/생성일/)).not.toBeInTheDocument();
    expect(container.querySelector('time')).toBeNull();
    expect(container.querySelector('.project-card')).not.toHaveAttribute('data-featured');
  });

  it('renders zero configs as "0 configs" without hiding the chip', () => {
    // LEFT JOIN surfaces zero-env projects on the backend; the card must
    // honour that by rendering the chip at 0, not collapsing it. "Missing
    // configs" is a 1급 visual signal per DESIGN.md principle 3.
    renderWithProviders(
      <ProjectCard project={{ ...sampleProject, env_count: 0 }} />,
    );
    expect(screen.getByText('0 configs')).toBeInTheDocument();
    expect(screen.getByLabelText('환경 0개')).toBeInTheDocument();
  });
});
