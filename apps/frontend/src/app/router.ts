import { useEffect, useState } from 'react';

/* Minimal History API router: no external dependency. nginx and Vite already
 * fall back to index.html for unknown paths. */
export const ROUTES = {
  root: '/',
  login: '/login',
  signup: '/signup',
  recoverPin: '/recover-pin',
  verifyEmail: '/verify-email',
  onboarding: '/onboarding',
  dashboardPrefix: '/dashboard/',
} as const;

export const dashboardPath = (contractorId: string) =>
  `${ROUTES.dashboardPrefix}${encodeURIComponent(contractorId)}`;

export type Route =
  | { readonly name: 'root' }
  | { readonly name: 'login' }
  | { readonly name: 'signup' }
  | { readonly name: 'recoverPin' }
  | { readonly name: 'verifyEmail' }
  | { readonly name: 'onboarding' }
  | { readonly name: 'dashboard'; readonly contractorId: string }
  | { readonly name: 'notFound' };

export function matchRoute(pathname: string): Route {
  if (pathname === ROUTES.root) return { name: 'root' };
  if (pathname === ROUTES.login) return { name: 'login' };
  if (pathname === ROUTES.signup) return { name: 'signup' };
  if (pathname === ROUTES.recoverPin) return { name: 'recoverPin' };
  if (pathname === ROUTES.verifyEmail) return { name: 'verifyEmail' };
  if (pathname === ROUTES.onboarding) return { name: 'onboarding' };
  if (pathname.startsWith(ROUTES.dashboardPrefix)) {
    const contractorId = decodeURIComponent(pathname.slice(ROUTES.dashboardPrefix.length));
    if (contractorId && !contractorId.includes('/')) return { name: 'dashboard', contractorId };
  }
  return { name: 'notFound' };
}

const NAVIGATION_EVENT = 'app:navigate';

export function navigate(path: string, { replace = false } = {}) {
  if (path === window.location.pathname) return;
  if (replace) window.history.replaceState(null, '', path);
  else window.history.pushState(null, '', path);
  window.dispatchEvent(new Event(NAVIGATION_EVENT));
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => matchRoute(window.location.pathname));
  useEffect(() => {
    const update = () => setRoute(matchRoute(window.location.pathname));
    window.addEventListener('popstate', update);
    window.addEventListener(NAVIGATION_EVENT, update);
    // Child effects (e.g. a <Redirect>) run before this one and may already
    // have navigated: sync once after subscribing.
    update();
    return () => {
      window.removeEventListener('popstate', update);
      window.removeEventListener(NAVIGATION_EVENT, update);
    };
  }, []);
  return route;
}
