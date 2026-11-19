import { useCallback, useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { ArrowUpRight, FileCheck2, LockKeyhole, Send, ShieldCheck } from 'lucide-react';
import { api } from '../api';
import { createDemoSubmission, demoUser } from '../demo';
import { AppLink } from '../router';
import type { Session, Submission } from '../types';
import { errorText, formatDate, formatDuration, formatTime } from '../utils';
import { VerificationBadge } from '../components/SessionTimeline';
import { CopyButton, EmptyState, ErrorState, InlineMessage, LoadingBlock, Modal } from '../components/ui';

export function Submissions({ sessions, demo }: { sessions: Session[]; demo: boolean }) {
  const [submissions, setSubmissions] = useState<Submission[]>(() => demo ? [createDemoSubmission('')] : []);
  const [loading, setLoading] = useState(!demo);
  const [loadError, setLoadError] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [note, setNote] = useState('');
  const [pending, setPending] = useState(false);
  const [createError, setCreateError] = useState('');
  const [shared, setShared] = useState<Submission | null>(null);

  const load = useCallback(async () => {
    if (demo) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError('');
    try {
      setSubmissions(await api.submissions());
    } catch (error) {
      setLoadError(errorText(error));
    } finally {
      setLoading(false);
    }
  }, [demo]);

  useEffect(() => {
    void load();
  }, [load]);

  const selectableSessions = useMemo(() => sessions.filter((session) => session.kind === 'engaged'), [sessions]);
  const selectedSessions = useMemo(() => selectableSessions.filter((session) => selected.has(session.id)), [selectableSessions, selected]);
  const selectedSeconds = selectedSessions.reduce((sum, session) => sum + session.seconds, 0);
  const allSelected = selectableSessions.length > 0 && selectedSessions.length === Math.min(100, selectableSessions.length);

  const toggle = (id: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else if (next.size >= 100) {
        setCreateError('A submission can include at most 100 sessions.');
        return current;
      } else next.add(id);
      setCreateError('');
      return next;
    });
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!selected.size) {
      setCreateError('Select at least one session to include in this submission.');
      return;
    }
    setPending(true);
    setCreateError('');
    try {
      let submission: Submission;
      if (demo) {
        submission = {
          id: `demo-submission-${Date.now()}`,
          author: demoUser.username,
          token: `sample-${Date.now().toString(36)}`,
          created_at: new Date().toISOString(),
          note: note.trim(),
          seconds: selectedSeconds,
          session_count: selectedSessions.length,
          status: 'ready for review',
          manifest_hash: Array.from({ length: 64 }, () => Math.floor(Math.random() * 16).toString(16)).join(''),
          sessions: selectedSessions,
        };
      } else {
        submission = await api.createSubmission([...selected], note.trim());
      }
      setSubmissions((current) => [submission, ...current]);
      setShared(submission);
      setSelected(new Set());
      setNote('');
    } catch (error) {
      setCreateError(errorText(error));
    } finally {
      setPending(false);
    }
  };

  const reviewerUrl = shared?.token ? `${window.location.origin}/review/${encodeURIComponent(shared.token)}${demo ? '?demo=1' : ''}` : '';

  return (
    <div className="page submissions-page">
      <header className="page-heading">
        <div><p className="eyebrow">CONTEXT, CURATED BY YOU</p><h1>Submissions & evidence</h1><p>Choose complete sessions, explain what changed, and invite thoughtful feedback.</p></div>
        <span className="heading-note"><ShieldCheck size={18} aria-hidden="true" />Nothing is shared automatically</span>
      </header>

      <section className="submission-explainer" aria-label="How evidence sharing works">
        <div><span>01</span><strong>Select</strong><p>Choose only the sessions that belong to this milestone.</p></div>
        <div><span>02</span><strong>Add context</strong><p>Explain your intent, iterations, and where feedback would help.</p></div>
        <div><span>03</span><strong>Share</strong><p>Create a read-only link with exactly those sessions.</p></div>
      </section>

      <div className="submission-workspace">
        <section className="session-selector" aria-labelledby="session-selector-title">
          <header className="section-heading-row">
            <div><p className="eyebrow">YOUR PRIVATE RECORD</p><h2 id="session-selector-title">Select sessions</h2></div>
            {selectableSessions.length > 0 && <button className="text-button" type="button" onClick={() => setSelected(allSelected ? new Set() : new Set(selectableSessions.slice(0, 100).map((session) => session.id)))}>{allSelected ? 'Clear all' : selectableSessions.length > 100 ? 'Select first 100' : 'Select all engaged'}</button>}
          </header>
          {sessions.length ? (
            <div className="selection-list">
              {sessions.map((session) => (
                <label className={`selection-row ${selected.has(session.id) ? 'selection-checked' : ''} ${session.kind === 'background_ai' ? 'selection-disabled' : ''}`} key={session.id}>
                  <input type="checkbox" checked={selected.has(session.id)} onChange={() => toggle(session.id)} disabled={session.kind === 'background_ai'} aria-label={`${session.kind === 'engaged' ? 'Include' : 'Not eligible:'} ${session.project_name}, ${formatDuration(session.seconds)} from ${formatDate(session.start_at)}`} />
                  <span className="selection-check" aria-hidden="true" />
                  <span className="selection-project-color" style={{ backgroundColor: session.project_color }} aria-hidden="true" />
                  <span className="selection-main"><strong>{session.project_name}</strong><small>{formatDate(session.start_at, { month: 'short', day: 'numeric' })} · {formatTime(session.start_at)} · {session.source || 'Local agent'}</small><span><VerificationBadge session={session} /></span></span>
                  <span className="selection-time"><strong>{formatDuration(session.seconds)}</strong><small className={`kind-text kind-text-${session.kind}`}>{session.kind === 'engaged' ? 'Engaged' : 'Background AI'}</small>{session.kind === 'background_ai' && <small>Not eligible for engaged-time submissions</small>}</span>
                </label>
              ))}
            </div>
          ) : (
            <EmptyState title="No sessions to share yet"><p>Make something with a connected device first. Your sessions will stay private until you select them here.</p><AppLink to="/settings" className="text-link">Connect a device <ArrowUpRight size={15} aria-hidden="true" /></AppLink></EmptyState>
          )}
        </section>

        <aside className="submission-composer" aria-labelledby="composer-title">
          <form className="stack-form" onSubmit={submit}>
            <div><p className="eyebrow">A LITTLE CONTEXT GOES A LONG WAY</p><h2 id="composer-title">Prepare submission</h2></div>
            <div className="selected-total" aria-live="polite"><div><strong>{selectedSessions.length}</strong><span>{selectedSessions.length === 1 ? 'session' : 'sessions'} selected</span></div><div><strong>{formatDuration(selectedSeconds)}</strong><span>total evidence</span></div></div>
            <div className="field"><label htmlFor="submission-note">Note for your reviewer <span className="optional">Optional</span></label><textarea id="submission-note" value={note} onChange={(event) => setNote(event.target.value)} rows={7} maxLength={2000} placeholder="What were you trying to make? What did you try, and where would feedback be most useful?" /><p className="field-hint">Up to 2,000 characters. AI interactions inside engaged sessions stay visible; standalone background AI sessions cannot be submitted as engaged time.</p></div>
            {createError && <InlineMessage tone="error">{createError}</InlineMessage>}
            <div className="share-assurance"><LockKeyhole size={17} aria-hidden="true" /><p>Only your <strong>{selectedSessions.length} selected {selectedSessions.length === 1 ? 'session' : 'sessions'}</strong> and this note will be included. Your other activity stays private.</p></div>
            <button className="button button-primary button-wide" type="submit" disabled={pending || !selectableSessions.length}><Send size={17} aria-hidden="true" />{pending ? 'Creating secure link…' : 'Create read-only link'}</button>
          </form>
        </aside>
      </div>

      <section className="submission-history" aria-labelledby="history-title">
        <header className="section-heading-row"><div><p className="eyebrow">SHARED, DELIBERATELY</p><h2 id="history-title">Submission history</h2></div><span className="plain-note">Reviewer URLs are shown only at creation</span></header>
        {loading ? <LoadingBlock label="Loading your submissions…" /> : loadError ? <ErrorState message={loadError} onRetry={() => void load()} /> : submissions.length ? (
          <div className="submission-list">
            {submissions.map((submission) => (
              <details className="submission-item" key={submission.id}>
                <summary>
                  <span className="submission-icon"><FileCheck2 size={20} aria-hidden="true" /></span>
                  <span className="submission-title"><strong>{submission.note ? submission.note.split(/[.!?]/)[0].slice(0, 80) : 'Selected work evidence'}</strong><small>Created {formatDate(submission.created_at)} · {submission.session_count} {submission.session_count === 1 ? 'session' : 'sessions'}</small></span>
                  <span className="submission-duration">{formatDuration(submission.seconds)}</span>
                  <span className="status-badge">{submission.status || 'ready'}</span>
                </summary>
                <div className="submission-detail">
                  {submission.note && <blockquote>{submission.note}</blockquote>}
                  <div className="submission-detail-sessions">{submission.sessions.map((session) => <span key={session.id}><i style={{ backgroundColor: session.project_color }} />{session.project_name}<b>{formatDuration(session.seconds)}</b></span>)}</div>
                  <div className="submission-integrity"><div><span>Manifest SHA-256</span><code title={submission.manifest_hash}>{submission.manifest_hash}</code></div><CopyButton value={submission.manifest_hash} label="Copy hash" /></div>
                  <p className="field-hint">This immutable snapshot is shared by @{submission.author}. Its reviewer URL was shown only when it was created and cannot be recovered here.</p>
                </div>
              </details>
            ))}
          </div>
        ) : <EmptyState title="No submissions yet"><p>When you want feedback on a milestone, select its sessions above. Each submission gets its own read-only link.</p></EmptyState>}
      </section>

      <Modal open={Boolean(shared?.token)} onClose={() => setShared(null)} title="Your evidence link is ready" eyebrow="SHARED ON YOUR TERMS" className="share-modal">
        {shared?.token && <div className="stack-form">
          <div className="share-success-mark"><FileCheck2 size={30} aria-hidden="true" /></div>
          <p className="modal-intro">This link contains <strong>{shared.session_count} {shared.session_count === 1 ? 'session' : 'sessions'}</strong> and {formatDuration(shared.seconds)} of evidence. Reviewers can read it without signing in; they cannot edit your record.</p>
          <div className="field"><label htmlFor="reviewer-link">Read-only reviewer link</label><div className="copy-field"><input id="reviewer-link" value={reviewerUrl} readOnly onFocus={(event) => event.target.select()} /><CopyButton value={reviewerUrl} label="Copy link" /></div></div>
          <div className="share-assurance"><LockKeyhole size={17} aria-hidden="true" /><p>Anyone with this link can view this submission. Share it only with people you trust. <strong>Copy it now:</strong> it cannot be recovered from submission history later.</p></div>
          <div className="modal-actions"><button className="button button-secondary" type="button" onClick={() => setShared(null)}>Done</button><AppLink to={`/review/${shared.token}`} className="button button-primary">Preview reviewer view <ArrowUpRight size={16} aria-hidden="true" /></AppLink></div>
        </div>}
      </Modal>
    </div>
  );
}
