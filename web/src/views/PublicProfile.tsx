import { useCallback, useEffect, useState } from 'react';
import { ArrowUpRight, CalendarDays, ExternalLink, FolderOpen, Globe2, LockKeyhole } from 'lucide-react';
import { api } from '../api';
import { createDemoDashboard, demoUser } from '../demo';
import type { Dashboard } from '../types';
import { errorText, formatDate, formatDuration, initials, safeExternalUrl } from '../utils';
import { ProjectAllocation, WeeklyChart } from '../components/Charts';
import { PublicHeader } from '../components/PublicHeader';
import { SessionTimeline } from '../components/SessionTimeline';
import { EmptyState, LoadingBlock } from '../components/ui';

export function PublicProfile({ username, demo }: { username: string; demo: boolean }) {
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      if (demo) {
        if (username !== demoUser.username) throw new Error('This sample profile is private or unavailable. Try the sample user “maya”.');
        const sample = createDemoDashboard();
        const publicProjects = sample.projects.filter((project) => project.is_public);
        const projectIds = new Set(publicProjects.map((project) => project.id));
        setDashboard({
          ...sample,
          projects: publicProjects,
          timeline: sample.timeline.filter((session) => projectIds.has(session.project_id)),
          stats: {
            ...sample.stats,
            total_seconds: publicProjects.reduce((sum, project) => sum + project.total_seconds, 0),
            today_seconds: publicProjects.reduce((sum, project) => sum + project.today_seconds, 0),
          },
        });
      } else {
        setDashboard(await api.publicProfile(username));
      }
    } catch (reason) {
      setDashboard(null);
      setError(errorText(reason));
    } finally {
      setLoading(false);
    }
  }, [username, demo]);

  useEffect(() => {
    void load();
  }, [load]);

  const publicProjects = dashboard?.projects.filter((project) => project.is_public) ?? [];
  const publicProjectIds = new Set(publicProjects.map((project) => project.id));
  const publicSessions = dashboard?.timeline.filter((session) => publicProjectIds.has(session.project_id)) ?? [];

  return (
    <div className="public-page">
      <a className="skip-link" href="#public-content">Skip to profile</a>
      <PublicHeader />
      <main id="public-content" className="public-main">
        {loading ? <LoadingBlock label="Loading public profile…" /> : error ? (
          <div className="public-unavailable"><LockKeyhole size={34} aria-hidden="true" /><p className="eyebrow">A PRIVATE THREAD</p><h1>This profile isn’t public.</h1><p>{error}</p><p>The owner chooses what to share. GritLoom does not expose private profile data.</p></div>
        ) : dashboard && (!dashboard.user.is_public ? (
          <div className="public-unavailable"><LockKeyhole size={34} aria-hidden="true" /><p className="eyebrow">A PRIVATE THREAD</p><h1>This profile isn’t public.</h1><p>The owner has chosen to keep this workspace private.</p></div>
        ) : (
          <>
            <section className="public-profile-hero" aria-labelledby="public-profile-name">
              <div className="public-avatar">{initials(dashboard.user.display_name)}</div>
              <div className="public-profile-copy"><p className="eyebrow"><Globe2 size={14} aria-hidden="true" />A PUBLIC MAKING RECORD</p><h1 id="public-profile-name">{dashboard.user.display_name}</h1><p className="public-username">@{dashboard.user.username}</p><p className="public-bio">{dashboard.user.bio || 'Learning in public, one project at a time.'}</p><span className="joined-date"><CalendarDays size={14} aria-hidden="true" />Building with GritLoom since {formatDate(dashboard.user.created_at, { month: 'long', year: 'numeric' })}</span></div>
              <div className="public-profile-note"><span>NOT A LEADERBOARD</span><p>A record of practice, experiments, and follow-through.</p></div>
            </section>

            <section className="public-metrics" aria-label="Public making statistics">
              <div><strong>{formatDuration(dashboard.stats.total_seconds)}</strong><span>engaged across public work</span></div>
              <div><strong>{formatDuration(dashboard.stats.week_seconds)}</strong><span>engaged this week</span></div>
              <div><strong>{dashboard.stats.streak_days} days</strong><span>current making rhythm</span></div>
              <div><strong>{dashboard.stats.iteration_count.toLocaleString()}</strong><span>iterations & learning loops</span></div>
            </section>

            <div className="public-dashboard-grid">
              <section className="panel" aria-labelledby="public-week-title"><header className="panel-header"><div><p className="eyebrow">THE LAST SEVEN DAYS</p><h2 id="public-week-title">A week of making</h2></div></header><WeeklyChart daily={dashboard.daily} /></section>
              <section className="panel" aria-labelledby="public-allocation-title"><header className="panel-header"><div><p className="eyebrow">PUBLIC PROJECTS</p><h2 id="public-allocation-title">Current threads</h2></div></header>{publicProjects.some((project) => project.total_seconds > 0) ? <ProjectAllocation projects={publicProjects} limit={4} /> : <EmptyState title="No public time yet"><p>Public project allocation will appear here once time has been recorded.</p></EmptyState>}</section>
            </div>

            <section className="public-projects" aria-labelledby="public-projects-title">
              <header className="section-heading-row"><div><p className="eyebrow">MADE WITH CURIOSITY</p><h2 id="public-projects-title">Projects</h2></div><span className="plain-note">Only projects marked public appear here</span></header>
              {publicProjects.length ? publicProjects.map((project) => {
                const repository = safeExternalUrl(project.repository_url);
                return <article className="public-project-row" key={project.id}><span className="project-color-block" style={{ backgroundColor: project.color }} aria-hidden="true" /><div><div className="project-name-line"><h3>{project.name}</h3>{project.language && <span className="language-tag">{project.language}</span>}</div><p>{project.description}</p>{repository && <a href={repository} target="_blank" rel="noreferrer" className="text-link">View repository <ExternalLink size={13} aria-hidden="true" /><span className="sr-only"> (opens in a new tab)</span></a>}</div><strong>{formatDuration(project.total_seconds)}</strong></article>;
              }) : <EmptyState icon={<FolderOpen size={24} aria-hidden="true" />} title="No public projects yet"><p>This builder is keeping their current threads private.</p></EmptyState>}
            </section>

            {publicSessions.length > 0 && <section className="panel public-timeline" aria-labelledby="public-activity-title"><header className="panel-header"><div><p className="eyebrow">PUBLIC EVIDENCE</p><h2 id="public-activity-title">Recent making activity</h2><p>Only sessions belonging to public projects are shown.</p></div></header><SessionTimeline sessions={publicSessions} compact /></section>}
          </>
        ))}
      </main>
      <footer className="public-footer"><span>GritLoom · self-improvement, not surveillance. · <a href="https://github.com/SabhyaAggarwal/gritloom" target="_blank" rel="noreferrer">AGPLv3 source</a></span><a href="/login">Make your own record <ArrowUpRight size={14} aria-hidden="true" /></a></footer>
    </div>
  );
}
