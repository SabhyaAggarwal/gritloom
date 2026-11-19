import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight, Bot, Flame, LockKeyhole, Minus, Plus, Sparkles } from 'lucide-react';
import type { Dashboard } from '../types';
import { AppLink } from '../router';
import { errorText, firstName, formatDuration, greeting } from '../utils';
import { ProjectAllocation, WeeklyChart } from '../components/Charts';
import { SessionTimeline } from '../components/SessionTimeline';
import { EmptyState, InlineMessage } from '../components/ui';

export function Overview({ dashboard, onUpdateGoal }: { dashboard: Dashboard; onUpdateGoal: (seconds: number) => Promise<void> }) {
  const { stats, user } = dashboard;
  const [goal, setGoal] = useState(stats.weekly_goal_seconds);
  const [goalPending, setGoalPending] = useState(false);
  const [goalError, setGoalError] = useState('');
  const [filter, setFilter] = useState<'all' | 'engaged' | 'background_ai'>('all');
  const progress = goal > 0 ? Math.min(100, stats.week_seconds / goal * 100) : 0;
  const remaining = Math.max(0, goal - stats.week_seconds);
  const filteredSessions = useMemo(() => filter === 'all' ? dashboard.timeline : dashboard.timeline.filter((session) => session.kind === filter), [dashboard.timeline, filter]);

  useEffect(() => setGoal(stats.weekly_goal_seconds), [stats.weekly_goal_seconds]);

  const updateGoal = async (next: number) => {
    const previous = goal;
    setGoal(next);
    setGoalPending(true);
    setGoalError('');
    try {
      await onUpdateGoal(next);
    } catch (error) {
      setGoal(previous);
      setGoalError(errorText(error));
    } finally {
      setGoalPending(false);
    }
  };

  return (
    <div className="page overview-page">
      <header className="page-heading overview-heading">
        <div>
          <p className="eyebrow">{new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date())}</p>
          <h1>{greeting()}, {firstName(user.display_name)}.</h1>
          <p>Here’s the shape of your making—not a judgment of it.</p>
        </div>
        <div className={`live-state ${dashboard.live ? 'is-live' : ''}`} role="status">
          <span className="live-dot" aria-hidden="true" />
          <div><strong>{dashboard.live ? 'Session in progress' : 'Between sessions'}</strong><small>{dashboard.live ? 'Your local agent is active' : 'A quiet moment counts, too'}</small></div>
        </div>
      </header>

      <section className="metric-strip" aria-label="Your time at a glance">
        <div className="metric metric-primary">
          <span className="metric-label">Engaged today</span>
          <strong>{formatDuration(stats.today_seconds)}</strong>
          <span className="metric-note">Hands-on making</span>
        </div>
        <div className="metric">
          <span className="metric-label">This week</span>
          <strong>{formatDuration(stats.week_seconds)}</strong>
          <span className="metric-note">Engaged time</span>
        </div>
        <div className="metric">
          <span className="metric-label"><Flame size={15} aria-hidden="true" />Current streak</span>
          <strong>{stats.streak_days}<small> days</small></strong>
          <span className="metric-note">Showing up counts</span>
        </div>
        <div className="metric">
          <span className="metric-label"><Sparkles size={15} aria-hidden="true" />Iterations</span>
          <strong>{stats.iteration_count.toLocaleString()}</strong>
          <span className="metric-note">Edits, tests & commits</span>
        </div>
      </section>

      <section className="ai-separation" aria-label="Background AI time">
        <div className="ai-separation-icon"><Bot size={22} aria-hidden="true" /></div>
        <div><strong>Background AI, kept in its own lane.</strong><p><b>{formatDuration(stats.background_ai_seconds)}</b> recorded separately, so assistance never inflates your engaged hours.</p></div>
        <span className="plain-note">{formatDuration(stats.ai_seconds)} AI-assisted engaged time</span>
      </section>

      <section className="goal-panel" aria-labelledby="weekly-goal-title">
        <div className="goal-copy">
          <p className="eyebrow">A RHYTHM, NOT A QUOTA</p>
          <h2 id="weekly-goal-title">Weekly engaged goal</h2>
          <p>{remaining > 0 ? `${formatDuration(remaining)} to your chosen rhythm. Rest is part of the process.` : 'Goal reached. Take a breath before deciding what comes next.'}</p>
        </div>
        <div className="goal-progress-wrap">
          <div className="goal-progress-label"><span>{formatDuration(stats.week_seconds)} woven</span><strong>{Math.round(stats.week_seconds / Math.max(1, goal) * 100)}%</strong></div>
          <div className="progress-track" role="progressbar" aria-label="Weekly engaged goal progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress)}><span style={{ width: `${progress}%` }} /></div>
        </div>
        <div className="goal-control">
          <span className="control-label" id="goal-control-label">Adjust your goal</span>
          <div className="stepper" role="group" aria-labelledby="goal-control-label">
            <button type="button" onClick={() => updateGoal(Math.max(1800, goal - 1800))} disabled={goalPending || goal <= 1800} aria-label="Decrease weekly goal by 30 minutes"><Minus size={17} aria-hidden="true" /></button>
            <output aria-live="polite" aria-label="Weekly goal">{formatDuration(goal)}<small> / week</small></output>
            <button type="button" onClick={() => updateGoal(Math.min(604800, goal + 1800))} disabled={goalPending || goal >= 604800} aria-label="Increase weekly goal by 30 minutes"><Plus size={17} aria-hidden="true" /></button>
          </div>
          {goalPending && <span className="field-hint">Saving…</span>}
        </div>
        {goalError && <div className="goal-error"><InlineMessage tone="error">{goalError}</InlineMessage></div>}
      </section>

      <div className="overview-grid">
        <section className="panel chart-panel" aria-labelledby="week-chart-heading">
          <header className="panel-header">
            <div><p className="eyebrow">THE LAST SEVEN DAYS</p><h2 id="week-chart-heading">Your making, day by day</h2></div>
            <span className="panel-side-note">Hours, clearly separated</span>
          </header>
          <WeeklyChart daily={dashboard.daily} />
        </section>
        <section className="panel allocation-panel" aria-labelledby="allocation-heading">
          <header className="panel-header">
            <div><p className="eyebrow">WHERE YOUR ENERGY WENT</p><h2 id="allocation-heading">Project allocation</h2></div>
          </header>
          {dashboard.projects.length && dashboard.projects.some((project) => project.total_seconds > 0) ? (
            <ProjectAllocation projects={dashboard.projects} />
          ) : (
            <EmptyState title="No project threads yet"><p>Once your local agent records a session, your projects will appear here.</p></EmptyState>
          )}
          <AppLink to="/projects" className="panel-footer-link">Explore projects <ArrowUpRight size={15} aria-hidden="true" /></AppLink>
        </section>
      </div>

      <section className="panel timeline-panel" aria-labelledby="timeline-heading">
        <header className="panel-header timeline-header">
          <div><p className="eyebrow">THE WORK BEHIND THE HOURS</p><h2 id="timeline-heading">Activity & evidence</h2><p>Newest first. Every thread includes its source and iteration trail.</p></div>
          <div className="compact-field"><label htmlFor="activity-filter">Show</label><select id="activity-filter" value={filter} onChange={(event) => setFilter(event.target.value as typeof filter)}><option value="all">All activity</option><option value="engaged">Engaged only</option><option value="background_ai">Background AI only</option></select></div>
        </header>
        {filteredSessions.length ? <SessionTimeline sessions={filteredSessions} /> : (
          <EmptyState title={dashboard.timeline.length ? 'No activity in this view' : 'Your first thread starts with a session'}>
            <p>{dashboard.timeline.length ? 'Choose another activity filter to see the rest of your record.' : 'Connect a device in Settings, then make something. Engaged work and background AI will appear here with clear evidence labels.'}</p>
            {!dashboard.timeline.length && <AppLink to="/settings" className="text-link">Set up a device <ArrowUpRight size={15} aria-hidden="true" /></AppLink>}
          </EmptyState>
        )}
      </section>

      <aside className="privacy-reassurance" aria-label="Privacy reassurance">
        <LockKeyhole size={23} aria-hidden="true" />
        <div><h2>Your record is not a surveillance feed.</h2><p>GritLoom is here to help you notice your own growth. Raw activity stays private unless you make your profile public or deliberately select sessions for a reviewer. Failed tests are evidence of learning—not marks against you.</p></div>
        <AppLink to="/settings" className="text-link">Review your privacy <ArrowUpRight size={15} aria-hidden="true" /></AppLink>
      </aside>
    </div>
  );
}
