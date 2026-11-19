import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api';
import { AppShell } from './components/Layout';
import { ErrorState, InlineMessage, LoadingBlock, Logo, Spinner } from './components/ui';
import { createDemoDashboard, demoUser } from './demo';
import { useRouter } from './router';
import type { ConnectionState, Dashboard, ProfileUpdate, Project, ProjectInput, User } from './types';
import { AuthView } from './views/AuthView';
import { NotFound } from './views/NotFound';
import { Overview } from './views/Overview';
import { Projects } from './views/Projects';
import { PublicProfile } from './views/PublicProfile';
import { ReviewSubmission } from './views/ReviewSubmission';
import { Settings } from './views/Settings';
import { Submissions } from './views/Submissions';

function useDashboard(enabled: boolean, demo: boolean, onUnauthorized: () => void) {
  const [data, setData] = useState<Dashboard | null>(() => demo ? createDemoDashboard() : null);
  const [loading, setLoading] = useState(enabled && !demo);
  const [error, setError] = useState<ApiError | null>(null);
  const [initialized, setInitialized] = useState(demo);
  const [connection, setConnection] = useState<ConnectionState>(demo ? 'demo' : 'idle');
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  const load = useCallback(async (initial = false) => {
    if (!enabled || demo) return;
    if (initial) {
      setLoading(true);
      setError(null);
      setConnection('idle');
    }
    try {
      const next = await api.dashboard();
      if (!mounted.current) return;
      setData(next);
      setError(null);
      setInitialized(true);
      if (initial) setConnection('connecting');
    } catch (reason) {
      if (!mounted.current) return;
      const nextError = reason instanceof ApiError ? reason : new ApiError('Could not load your dashboard. Please try again.');
      setError(nextError);
      if (initial) setInitialized(false);
      if (nextError.status === 401) {
        setData(null);
        setConnection('idle');
        onUnauthorized();
      }
    } finally {
      if (mounted.current) setLoading(false);
    }
  }, [enabled, demo, onUnauthorized]);

  useEffect(() => {
    if (demo) {
      setConnection('demo');
      setInitialized(true);
      setLoading(false);
      return;
    }
    if (!enabled) {
      setLoading(false);
      setConnection('idle');
      return;
    }
    void load(true);
  }, [enabled, demo, load]);

  useEffect(() => {
    if (!enabled || demo || !initialized) return;
    let cancelled = false;
    let poll: number | undefined;
    const stream = new EventSource('/api/v1/events/stream', { withCredentials: true });
    setConnection('connecting');

    const startPolling = () => {
      if (cancelled) return;
      stream.close();
      setConnection('polling');
      void load(false);
      poll = window.setInterval(() => void load(false), 30000);
    };

    stream.onopen = () => {
      if (!cancelled) setConnection('live');
    };
    stream.addEventListener('dashboard', (event) => {
      if (cancelled) return;
      try {
        const next = JSON.parse((event as MessageEvent<string>).data) as Dashboard;
        setData(next);
        setError(null);
      } catch {
        setError(new ApiError('A live update could not be read. Your last good dashboard is still shown.'));
      }
    });
    stream.onerror = startPolling;

    return () => {
      cancelled = true;
      stream.close();
      if (poll !== undefined) window.clearInterval(poll);
    };
  }, [enabled, demo, initialized, load]);

  const clear = useCallback(() => {
    setData(null);
    setError(null);
    setInitialized(false);
    setLoading(false);
    setConnection('idle');
  }, []);

  return { data, setData, loading, error, connection, reload: () => load(true), clear };
}

function decoded(value: string): string | null {
  try {
    return decodeURIComponent(value);
  } catch {
    return null;
  }
}

function FullPageLoading() {
  return <main className="full-page-state"><Logo /><Spinner label="Opening your workspace…" /></main>;
}

export default function App() {
  const { path, demo, navigate } = useRouter();
  const [user, setUser] = useState<User | null>(() => demo ? { ...demoUser } : null);
  const [signedOut, setSignedOut] = useState(false);
  const publicMatch = path.match(/^\/u\/([^/]+)\/?$/);
  const reviewMatch = path.match(/^\/review\/([^/]+)\/?$/);
  const publicUsername = publicMatch ? decoded(publicMatch[1]) : null;
  const reviewToken = reviewMatch ? decoded(reviewMatch[1]) : null;
  const isPublicRoute = Boolean(publicMatch || reviewMatch);
  const privatePaths = ['/', '/projects', '/submissions', '/settings', '/login', '/register'];
  const isKnownPrivatePath = privatePaths.includes(path);
  const dashboardEnabled = !demo && !isPublicRoute && isKnownPrivatePath && !signedOut;
  const onUnauthorized = useCallback(() => {
    setUser(null);
    setSignedOut(true);
  }, []);
  const dashboard = useDashboard(dashboardEnabled, demo, onUnauthorized);
  const activeUser = dashboard.data?.user ?? user;

  useEffect(() => {
    if (activeUser && (path === '/login' || path === '/register')) navigate('/', true);
  }, [activeUser, path, navigate]);

  const authenticate = (authenticatedUser: User) => {
    setUser(authenticatedUser);
    setSignedOut(false);
    navigate('/');
  };

  const logout = async () => {
    if (demo) {
      window.location.assign('/login');
      return;
    }
    await api.logout();
    dashboard.clear();
    setUser(null);
    setSignedOut(true);
    navigate('/login');
  };

  const saveProfile = async (input: ProfileUpdate) => {
    if (!activeUser) return;
    const updated = demo ? {
      ...activeUser,
      display_name: input.display_name,
      bio: input.bio,
      is_public: input.is_public,
      timezone: input.timezone,
    } : await api.patchMe(input);
    setUser(updated);
    dashboard.setData((current) => current ? {
      ...current,
      user: updated,
      stats: { ...current.stats, weekly_goal_seconds: input.weekly_goal_seconds },
    } : current);
  };

  const updateGoal = async (weeklyGoalSeconds: number) => {
    if (!activeUser || !dashboard.data) return;
    await saveProfile({
      display_name: activeUser.display_name,
      bio: activeUser.bio,
      is_public: activeUser.is_public,
      weekly_goal_seconds: weeklyGoalSeconds,
      timezone: activeUser.timezone,
    });
  };

  const createProject = async (input: ProjectInput): Promise<Project> => {
    const created = demo ? {
      ...input,
      id: `demo-project-${Date.now()}`,
      slug: input.name.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''),
      language: '',
      total_seconds: 0,
      today_seconds: 0,
      last_active_at: new Date().toISOString(),
      is_public: true,
    } : await api.createProject(input);
    dashboard.setData((current) => current ? {
      ...current,
      projects: current.projects.some((project) => project.id === created.id)
        ? current.projects.map((project) => project.id === created.id ? created : project)
        : [...current.projects, created],
    } : current);
    return created;
  };

  const updateProjectVisibility = async (projectID: string, isPublic: boolean): Promise<Project> => {
    const currentProject = dashboard.data?.projects.find((project) => project.id === projectID);
    if (!currentProject) throw new ApiError('This project is no longer in your workspace. Reload and try again.');
    const updated = demo ? { ...currentProject, is_public: isPublic } : await api.setProjectVisibility(projectID, isPublic);
    dashboard.setData((current) => current ? {
      ...current,
      projects: current.projects.map((project) => project.id === projectID ? updated : project),
    } : current);
    return updated;
  };

  if (publicMatch) {
    return publicUsername ? <PublicProfile username={publicUsername} demo={demo} /> : <NotFound />;
  }
  if (reviewMatch) {
    return reviewToken ? <ReviewSubmission token={reviewToken} demo={demo} /> : <NotFound />;
  }
  if (!isKnownPrivatePath) return <NotFound />;

  if (!activeUser) {
    if (signedOut && (path === '/login' || path === '/register' || dashboard.error?.status === 401)) {
      return <AuthView mode={path === '/register' ? 'register' : 'login'} onAuthenticated={authenticate} />;
    }
    if (dashboard.error && dashboard.error.status !== 401) {
      return <main className="full-page-state"><Logo /><ErrorState title="Your workspace couldn’t open" message={dashboard.error.message} onRetry={dashboard.reload} /><p className="plain-note">GritLoom never substitutes sample data when the server is unavailable.</p></main>;
    }
    if (signedOut || dashboard.error?.status === 401) {
      return <AuthView mode={path === '/register' ? 'register' : 'login'} onAuthenticated={authenticate} />;
    }
    return <FullPageLoading />;
  }

  const page = (() => {
    if (!dashboard.data) return dashboard.loading ? <LoadingBlock /> : <ErrorState message={dashboard.error?.message ?? 'Your dashboard is not available yet.'} onRetry={dashboard.reload} />;
    if (path === '/projects') return <Projects projects={dashboard.data.projects} onCreate={createProject} onUpdateVisibility={updateProjectVisibility} />;
    if (path === '/submissions') return <Submissions sessions={dashboard.data.timeline} demo={demo} />;
    if (path === '/settings') return <Settings user={activeUser} weeklyGoalSeconds={dashboard.data.stats.weekly_goal_seconds} demo={demo} onSave={saveProfile} />;
    return <Overview dashboard={dashboard.data} onUpdateGoal={updateGoal} />;
  })();

  return (
    <AppShell user={activeUser} connection={dashboard.connection} live={dashboard.data?.live ?? false} onLogout={logout}>
      {demo && <div className="demo-banner" role="status"><span><strong>Demo workspace</strong> · Realistic sample data, no API requests.</span><a href="/">Exit demo</a></div>}
      {!demo && dashboard.error && dashboard.data && <InlineMessage tone="error">{dashboard.error.message} <button className="text-button" type="button" onClick={dashboard.reload}>Retry</button></InlineMessage>}
      {page}
    </AppShell>
  );
}
