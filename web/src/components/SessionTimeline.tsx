import { Bot, CheckCheck, Code2, Eye, GitCommitHorizontal, PencilLine, ShieldCheck, TestTube2 } from 'lucide-react';
import type { EvidenceEvent, Session } from '../types';
import { formatDuration, formatTime, formatDate } from '../utils';

function EvidenceDetails({ session }: { session: Session }) {
  if (session.evidence === undefined) return null;
  const events = session.evidence;
  const eventLabel = (event: EvidenceEvent) => event.kind.replace(/_/g, ' ');
  return (
    <details className="evidence-events">
      <summary><ShieldCheck size={15} aria-hidden="true" /><span>{events.length} granular evidence {events.length === 1 ? 'event' : 'events'}</span>{session.evidence_hash && <code title={`Session evidence hash: ${session.evidence_hash}`}>#{session.evidence_hash.slice(0, 10)}</code>}</summary>
      {events.length ? <ol>
        {events.map((event) => (
          <li key={event.id}>
            <div className="evidence-event-topline"><time dateTime={event.occurred_at}>{formatTime(event.occurred_at)}</time><strong>{eventLabel(event)}</strong><span>{event.source}</span>{event.ai && <em className="event-ai-label"><Bot size={12} aria-hidden="true" />{event.background ? 'Background AI' : 'AI assisted'}{event.model ? ` · ${event.model}` : ''}</em>}</div>
            <div className="evidence-event-detail">
              {event.file && <code className="event-file">{event.file}</code>}
              {(event.additions > 0 || event.deletions > 0) && <span className="line-changes"><b>+{event.additions}</b>{' '}<i>−{event.deletions}</i></span>}
              {event.exit_code !== undefined && <span className={event.exit_code === 0 ? 'exit-passed' : 'exit-failed'}>exit {event.exit_code}</span>}
              {event.commit_hash && <code title={`Commit ${event.commit_hash}`}>commit {event.commit_hash.slice(0, 10)}</code>}
              {event.branch && <span>branch {event.branch}</span>}
            </div>
          </li>
        ))}
      </ol> : <p className="evidence-empty">No granular events were included in this snapshot.</p>}
      {session.evidence_hash && <p className="evidence-hash-note">Evidence hash <code>{session.evidence_hash}</code></p>}
    </details>
  );
}

export const verificationLabels = {
  observed: 'Observed locally',
  corroborated: 'Corroborated',
  review_ready: 'Review-ready',
} as const;

export function VerificationBadge({ session }: { session: Session }) {
  const Icon = session.verification === 'observed' ? Eye : session.verification === 'corroborated' ? CheckCheck : ShieldCheck;
  return <span className={`evidence-badge evidence-${session.verification}`}><Icon size={13} aria-hidden="true" />{verificationLabels[session.verification] ?? session.verification}</span>;
}

export function SessionTimeline({ sessions, compact = false, chronological = false }: { sessions: Session[]; compact?: boolean; chronological?: boolean }) {
  const ordered = [...sessions].sort((a, b) => {
    const difference = new Date(b.start_at).getTime() - new Date(a.start_at).getTime();
    return chronological ? -difference : difference;
  });

  return (
    <ol className={`session-timeline ${compact ? 'timeline-compact' : ''}`}>
      {ordered.map((session, index) => {
        const start = new Date(session.start_at);
        const previous = ordered[index - 1] ? new Date(ordered[index - 1].start_at) : null;
        const newDay = index === 0 || !previous || start.toDateString() !== previous.toDateString();
        return (
          <li key={session.id} className="timeline-item">
            {newDay && <div className="timeline-date">{formatDate(session.start_at, { weekday: 'long', month: 'long', day: 'numeric' })}</div>}
            <div className="timeline-rail" aria-hidden="true"><span style={{ borderColor: session.project_color }} /><i /></div>
            <article className="timeline-session" aria-label={`${session.project_name}, ${formatDuration(session.seconds)} ${session.kind === 'engaged' ? 'engaged work' : 'background AI'}`}>
              <div className="session-topline">
                <div className="session-time">
                  <time dateTime={session.start_at}>{formatTime(session.start_at)}</time>
                  <span aria-hidden="true">→</span>
                  <time dateTime={session.end_at}>{formatTime(session.end_at)}</time>
                </div>
                <span className={`kind-badge kind-${session.kind}`}>
                  {session.kind === 'engaged' ? <Code2 size={13} aria-hidden="true" /> : <Bot size={13} aria-hidden="true" />}
                  {session.kind === 'engaged' ? 'Engaged' : 'Background AI'}
                </span>
              </div>
              <div className="session-title-row">
                <h3><span className="project-dot" style={{ backgroundColor: session.project_color }} aria-hidden="true" />{session.project_name}</h3>
                <strong className="session-duration">{formatDuration(session.seconds)}</strong>
              </div>
              <div className="session-evidence-row">
                <VerificationBadge session={session} />
                <span className="session-source">{session.source || 'Local agent'}</span>
              </div>
              {!compact && (
                <dl className="iteration-list" aria-label="Iteration evidence">
                  <div title="File edits"><dt><PencilLine size={14} aria-hidden="true" /><span className="sr-only">Edits</span></dt><dd>{session.edit_count}<span> edits</span></dd></div>
                  <div title="AI interactions"><dt><Bot size={14} aria-hidden="true" /><span className="sr-only">AI interactions</span></dt><dd>{session.ai_count}<span> AI</span></dd></div>
                  <div title="Tests run"><dt><TestTube2 size={14} aria-hidden="true" /><span className="sr-only">Tests</span></dt><dd>{session.test_count}<span> tests</span>{session.failed_test_count > 0 && <em>{session.failed_test_count} failed</em>}</dd></div>
                  <div title="Commits"><dt><GitCommitHorizontal size={14} aria-hidden="true" /><span className="sr-only">Commits</span></dt><dd>{session.commit_count}<span> commits</span></dd></div>
                </dl>
              )}
              <EvidenceDetails session={session} />
            </article>
          </li>
        );
      })}
    </ol>
  );
}
