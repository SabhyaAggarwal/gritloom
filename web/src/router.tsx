import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { AnchorHTMLAttributes, MouseEvent, ReactNode } from 'react';

interface RouterValue {
  path: string;
  demo: boolean;
  navigate: (to: string, replace?: boolean) => void;
}

const RouterContext = createContext<RouterValue | null>(null);

function currentPath(): string {
  return window.location.pathname || '/';
}

export function RouterProvider({ children }: { children: ReactNode }) {
  const [path, setPath] = useState(currentPath);
  const [demo, setDemo] = useState(() => new URLSearchParams(window.location.search).get('demo') === '1');

  useEffect(() => {
    const onPopState = () => {
      setPath(currentPath());
      setDemo(new URLSearchParams(window.location.search).get('demo') === '1');
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const navigate = useCallback((to: string, replace = false) => {
    const url = new URL(to, window.location.origin);
    if (new URLSearchParams(window.location.search).get('demo') === '1') url.searchParams.set('demo', '1');
    window.history[replace ? 'replaceState' : 'pushState']({}, '', `${url.pathname}${url.search}${url.hash}`);
    setPath(url.pathname);
    setDemo(url.searchParams.get('demo') === '1');
    window.scrollTo({ top: 0, behavior: 'auto' });
  }, []);

  const value = useMemo(() => ({ path, demo, navigate }), [path, demo, navigate]);
  return <RouterContext.Provider value={value}>{children}</RouterContext.Provider>;
}

export function useRouter(): RouterValue {
  const value = useContext(RouterContext);
  if (!value) throw new Error('useRouter must be used inside RouterProvider');
  return value;
}

interface AppLinkProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
  to: string;
}

export function AppLink({ to, onClick, children, ...props }: AppLinkProps) {
  const { navigate, demo } = useRouter();
  const url = new URL(to, window.location.origin);
  if (demo) url.searchParams.set('demo', '1');
  const href = `${url.pathname}${url.search}${url.hash}`;

  const follow = (event: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(event);
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      props.target === '_blank'
    ) return;
    event.preventDefault();
    navigate(to);
  };

  return <a {...props} href={href} onClick={follow}>{children}</a>;
}
