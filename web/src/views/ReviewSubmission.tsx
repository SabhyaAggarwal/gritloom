import { useCallback, useEffect, useState } from 'react';
import { ArrowUpRight, Bot, CheckCheck, Clock3, Eye, FileCheck2, LockKeyhole, ShieldCheck } from 'lucide-react';
import { api } from '../api';
import { createDemoSubmission } from '../demo';
import type { Submission } from '../types';
import { errorText, formatDate, formatDuration } from '../utils';
import { PublicHeader } from '../components/PublicHeader';
import { SessionTimeline } from '../components/SessionTimeline';
import { CopyButton, EmptyState, ErrorState, LoadingBlock } from '../components/ui';

export function ReviewSubmission({ token, demo }: { token: string; demo: boolean }) {
  const [submission, setSubmission] = useState<Submission | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setSubmission(demo ? createDemoSubmission(token) : await api.submission(token));
    } catch (reason) {
      setSubmission(null);
      setError(errorText(reason));
    } finally {
      setLoading(false);
    }
  }, [token, demo]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="public-page review-page">
      <a className="skip-link" href="#review-content">Skip to submission</a>
      <PublicHeader />
      <main id="review-content" className="review-main">
        {loading ? <LoadingBlock label="Opening read-only submission…" /> : error ? <ErrorState title="This submission isn’t available" message={error} onRetry={() => void load()} /> : submission && (
          <>
            <header className="review-heading">
              <div><p className="eyebrow"><FileCheck2 size={15} aria-hidden="true" />READ-ONLY EVIDENCE REVIEW</p><h1>A milestone, with its learning left in.</h1><p><strong>@{submission.author}</strong> selected these sessions and wrote the context below. This is a snapshot for feedback—not a measure of their worth or potential.</p></div>
              <div className="read-only-seal"><LockKeyhole size={22} aria-hidden="true" /><strong>Read-only</strong><span>No sign-in required</span></div>
            </header>

            <section className="review-summary" aria-label="Submission summary">
              <div><Clock3 size={18} aria-hidden="true" /><span><strong>{formatDuration(submission.seconds)}</strong><small>total selected evidence</small></span></div>
              <div><FileCheck2 size={18} aria-hidden="true" /><span><strong>{submission.session_count}</strong><small>{submission.session_count === 1 ? 'selected session' : 'selected sessions'}</small></span></div>
              <div><Eye size={18} aria-hidden="true" /><span><strong>{formatDate(submission.created_at)}</strong><small>shared by the builder</small></span></div>
              <span className="status-badge">{submission.status || 'ready for review'}</span>
            </section>

            <section className="reviewer-note" aria-labelledby="reviewer-note-title">
              <div className="reviewer-note-label"><span>FROM THE BUILDER</span><h2 id="reviewer-note-title">Context for your review</h2></div>
              {submission.note ? <blockquote>{submission.note}</blockquote> : <p className="muted">The builder did not add a note. The selected sessions below are the complete submission.</p>}
            </section>

            <section className="submission-manifest" aria-label="Submission integrity metadata">
              <div><ShieldCheck size={19} aria-hidden="true" /><span><strong>Immutable evidence manifest</strong><small>SHA-256 identifies the exact note, author, timestamp, and session snapshots shared here.</small></span></div>
              <code>{submission.manifest_hash}</code>
              <CopyButton value={submission.manifest_hash} label="Copy manifest hash" />
              {!demo && <a className="text-link manifest-download" href={`/api/v1/submissions/${encodeURIComponent(token)}/manifest`} download>Download raw manifest <ArrowUpRight size={14} aria-hidden="true" /></a>}
            </section>

            <section className="panel review-sessions" aria-labelledby="review-sessions-title">
              <header className="panel-header"><div><p className="eyebrow">CHRONOLOGICAL, OLDEST FIRST</p><h2 id="review-sessions-title">The selected work</h2><p>These selected engaged sessions include their granular evidence events. Counts are evidence of iteration, not grades.</p></div></header>
              {submission.sessions.length ? <SessionTimeline sessions={submission.sessions} chronological /> : <EmptyState title="No sessions in this snapshot"><p>This submission does not include any activity sessions.</p></EmptyState>}
            </section>

            <aside className="evidence-key" aria-labelledby="evidence-key-title">
              <div><p className="eyebrow">HOW TO READ THE LABELS</p><h2 id="evidence-key-title">Evidence, with context</h2></div>
              <dl>
                <div><dt><Eye size={17} aria-hidden="true" />Observed locally</dt><dd>A local agent observed this work session.</dd></div>
                <div><dt><CheckCheck size={17} aria-hidden="true" />Corroborated</dt><dd>Multiple local signals support the session record.</dd></div>
                <div><dt><ShieldCheck size={17} aria-hidden="true" />Review-ready</dt><dd>The session has a complete evidence trail for review.</dd></div>
                <div><dt><Bot size={17} aria-hidden="true" />Background AI</dt><dd>AI ran in the background; this time is not engaged making.</dd></div>
              </dl>
            </aside>
          </>
        )}
      </main>
      <footer className="public-footer"><span>Shared deliberately with GritLoom. No other private sessions are included. · <a href="https://github.com/SabhyaAggarwal/gritloom" target="_blank" rel="noreferrer">AGPLv3 source</a></span><a href="/login">Build your own learning record <ArrowUpRight size={14} aria-hidden="true" /></a></footer>
    </div>
  );
}
