import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useParams,
  useSearch,
} from '@tanstack/react-router';

import { isAuthenticated } from './lib/auth';
import { ActionsPage } from './pages/Actions';
import { AuditPage } from './pages/Audit';
import { EnvDiffPage } from './pages/EnvDiff';
import { EnvSecretsPage } from './pages/EnvSecrets';
import { LoginPage } from './pages/Login';
import { ProjectPage } from './pages/Project';
import { ProjectsPage } from './pages/Projects';
import { SessionsPage } from './pages/Sessions';
import { TokensPage } from './pages/Tokens';
import { WebhooksPage } from './pages/Webhooks';

/**
 * Code-based router. Routes:
 *   /                                          → Projects list
 *   /projects/$project                         → Project detail (envs)
 *   /projects/$project/envs/$env               → Env secrets table + history
 *   /projects/$project/envs/$env/diff          → Env-vs-env diff (?against=<rhs>)
 *   /audit                                     → Audit feed (?project=&env=&actor=&action=)
 *   /login                                     → Login form
 *
 * defaultPreload='intent' starts loaders on hover/focus so navigation
 * feels instant after the operator scans the project/env grid.
 *
 * Route guards (beforeLoad):
 *   - Protected routes throw redirect({ to: '/login' }) when there is
 *     no CSRF token. TanStack Router treats the thrown redirect as a
 *     pre-render hop and never mounts the protected component, so the
 *     dashboard shell never makes an authenticated fetch from an
 *     unauthenticated context.
 *   - /login does the inverse: already-logged-in operators bounce home.
 */

function requireAuth() {
  if (!isAuthenticated()) {
    throw redirect({ to: '/login' });
  }
}

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: requireAuth,
  component: ProjectsPage,
});

const projectRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects/$project',
  beforeLoad: requireAuth,
  component: ProjectRouteComponent,
});

const envRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects/$project/envs/$env',
  beforeLoad: requireAuth,
  component: EnvRouteComponent,
});

const envDiffRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects/$project/envs/$env/diff',
  beforeLoad: requireAuth,
  // Drop empty strings so navigation without a selected rhs renders as
  // /diff (no `?against=`) instead of /diff?against=. EnvDiffPage treats
  // undefined as "not yet chosen" and shows the picker prompt.
  validateSearch: (search: Record<string, unknown>): { against?: string } => {
    if (typeof search.against === 'string' && search.against !== '') {
      return { against: search.against };
    }
    return {};
  },
  component: EnvDiffRouteComponent,
});

const auditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/audit',
  beforeLoad: requireAuth,
  validateSearch: (
    search: Record<string, unknown>,
  ): { project?: string; env?: string; actor?: number; action?: string } => {
    const out: { project?: string; env?: string; actor?: number; action?: string } = {};
    if (typeof search.project === 'string' && search.project !== '') out.project = search.project;
    if (typeof search.env === 'string' && search.env !== '') out.env = search.env;
    if (typeof search.action === 'string' && search.action !== '') out.action = search.action;
    if (typeof search.actor === 'number' && Number.isInteger(search.actor) && search.actor > 0) {
      out.actor = search.actor;
    } else if (typeof search.actor === 'string') {
      const parsed = Number(search.actor);
      if (Number.isInteger(parsed) && parsed > 0) out.actor = parsed;
    }
    return out;
  },
  component: AuditRouteComponent,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: () => {
    if (isAuthenticated()) {
      throw redirect({ to: '/' });
    }
  },
  component: LoginPage,
});

const settingsSessionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings/sessions',
  beforeLoad: requireAuth,
  component: SessionsPage,
});

const settingsTokensRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings/tokens',
  beforeLoad: requireAuth,
  component: TokensPage,
});

const integrationsActionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/integrations/github-actions',
  beforeLoad: requireAuth,
  component: ActionsPage,
});

const integrationsWebhooksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/integrations/webhooks',
  beforeLoad: requireAuth,
  component: WebhooksPage,
});

// eslint-disable-next-line react-refresh/only-export-components
function ProjectRouteComponent() {
  const { project } = useParams({ from: '/projects/$project' });
  return <ProjectPage projectName={project} />;
}

// eslint-disable-next-line react-refresh/only-export-components
function EnvRouteComponent() {
  const { project, env } = useParams({ from: '/projects/$project/envs/$env' });
  return <EnvSecretsPage projectName={project} envName={env} />;
}

// eslint-disable-next-line react-refresh/only-export-components
function EnvDiffRouteComponent() {
  const { project, env } = useParams({ from: '/projects/$project/envs/$env/diff' });
  const { against } = useSearch({ from: '/projects/$project/envs/$env/diff' });
  return <EnvDiffPage projectName={project} envName={env} against={against} />;
}

// eslint-disable-next-line react-refresh/only-export-components
function AuditRouteComponent() {
  const search = useSearch({ from: '/audit' });
  return <AuditPage filter={search} />;
}

const routeTree = rootRoute.addChildren([
  indexRoute,
  projectRoute,
  envRoute,
  envDiffRoute,
  auditRoute,
  loginRoute,
  settingsSessionsRoute,
  settingsTokensRoute,
  integrationsActionsRoute,
  integrationsWebhooksRoute,
]);

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
