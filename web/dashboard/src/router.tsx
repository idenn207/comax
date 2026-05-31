import { Outlet, createRootRoute, createRoute, createRouter } from '@tanstack/react-router';

import { HomePage } from './pages/Home';
import { LoginPage } from './pages/Login';

/**
 * Code-based router for the scaffold. Two routes (/, /login) are the
 * full surface for Task 5; migrating to file-based routing makes sense
 * once Task 7+ adds nested project/env/secrets routes that benefit from
 * the colocation.
 *
 * defaultPreload='intent' starts loaders on hover/focus so navigation
 * feels instant once Task 7 introduces per-route queries.
 */

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
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
