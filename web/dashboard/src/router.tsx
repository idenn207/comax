import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useParams,
} from '@tanstack/react-router';

import { isAuthenticated } from './lib/auth';
import { EnvSecretsPage } from './pages/EnvSecrets';
import { LoginPage } from './pages/Login';
import { ProjectPage } from './pages/Project';
import { ProjectsPage } from './pages/Projects';

/**
 * Code-based router. Routes:
 *   /                                    → Projects list
 *   /projects/$project                   → Project detail (envs)
 *   /projects/$project/envs/$env         → Env secrets table + history
 *   /login                               → Login form
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

const routeTree = rootRoute.addChildren([indexRoute, projectRoute, envRoute, loginRoute]);

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
