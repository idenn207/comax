import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router';

import { isAuthenticated } from './lib/auth';
import { HomePage } from './pages/Home';
import { LoginPage } from './pages/Login';

/**
 * Code-based router for the scaffold. Two routes (/, /login) cover the
 * Task 6 surface; migrating to file-based routing makes sense once Task
 * 7+ adds nested project/env/secrets routes that benefit from the
 * colocation.
 *
 * defaultPreload='intent' starts loaders on hover/focus so navigation
 * feels instant once Task 7 introduces per-route queries.
 *
 * Route guards (beforeLoad):
 *   - Protected routes throw redirect({ to: '/login' }) when there is
 *     no CSRF token. TanStack Router treats the thrown redirect as a
 *     pre-render hop and never mounts the protected component, so the
 *     dashboard shell never makes an authenticated fetch from an
 *     unauthenticated context.
 *   - /login does the inverse: if the operator is already logged in,
 *     punt them to / so back-button doesn't strand them on a useless
 *     form.
 */

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    if (!isAuthenticated()) {
      throw redirect({ to: '/login' });
    }
  },
  component: HomePage,
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

const routeTree = rootRoute.addChildren([indexRoute, loginRoute]);

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
