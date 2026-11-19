import { useState } from 'react';
import type { FormEvent } from 'react';
import { ExternalLink, FolderPlus, Globe2, LockKeyhole, Plus } from 'lucide-react';
import type { Project, ProjectInput } from '../types';
import { errorText, formatDuration, relativeTime, safeExternalUrl } from '../utils';
import { EmptyState, InlineMessage, Modal } from '../components/ui';

const emptyForm: ProjectInput = { name: '', repository_url: '', description: '', color: '#2f6a4f' };

export function Projects({ projects, onCreate, onUpdateVisibility }: { projects: Project[]; onCreate: (input: ProjectInput) => Promise<Project>; onUpdateVisibility: (projectID: string, isPublic: boolean) => Promise<Project> }) {
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<ProjectInput>(emptyForm);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [created, setCreated] = useState('');
  const [visibilityPending, setVisibilityPending] = useState<string | null>(null);
  const [visibilityError, setVisibilityError] = useState('');
  const total = projects.reduce((sum, project) => sum + project.total_seconds, 0);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setPending(true);
    setError('');
    try {
      const project = await onCreate({ ...form, name: form.name.trim(), description: form.description.trim(), repository_url: form.repository_url.trim() });
      setCreated(project.name);
      setForm(emptyForm);
      setOpen(false);
    } catch (reason) {
      setError(errorText(reason));
    } finally {
      setPending(false);
    }
  };

  const toggleVisibility = async (project: Project) => {
    setVisibilityPending(project.id);
    setVisibilityError('');
    try {
      await onUpdateVisibility(project.id, !project.is_public);
    } catch (reason) {
      setVisibilityError(errorText(reason));
    } finally {
      setVisibilityPending(null);
    }
  };

  return (
    <div className="page projects-page">
      <header className="page-heading">
        <div><p className="eyebrow">THE THREADS YOU’RE WEAVING</p><h1>Projects</h1><p>A home for each curiosity, with time and evidence in context.</p></div>
        <button className="button button-primary" type="button" onClick={() => { setOpen(true); setCreated(''); }}><Plus size={18} aria-hidden="true" />New project</button>
      </header>

      {created && <InlineMessage tone="success"><strong>{created}</strong> is ready for its first session.</InlineMessage>}
      {visibilityError && <InlineMessage tone="error">{visibilityError}</InlineMessage>}

      <section className="project-summary" aria-label="Project summary">
        <div><strong>{projects.length}</strong><span>{projects.length === 1 ? 'project' : 'projects'}</span></div>
        <div><strong>{formatDuration(total)}</strong><span>engaged across your work</span></div>
        <p>Project totals describe your record so far. They aren’t rankings.</p>
      </section>

      <section className="project-ledger" aria-label="Your projects">
        <div className="ledger-heading" aria-hidden="true"><span>Project</span><span>Today</span><span>All time</span><span>Last active</span></div>
        {projects.length ? [...projects].sort((a, b) => (b.last_active_at ? new Date(b.last_active_at).getTime() : 0) - (a.last_active_at ? new Date(a.last_active_at).getTime() : 0)).map((project) => {
          const repository = safeExternalUrl(project.repository_url);
          return (
            <article className="project-row" key={project.id}>
              <div className="project-main">
                <span className="project-color-block" style={{ backgroundColor: project.color }} aria-hidden="true" />
                <div>
                  <div className="project-name-line"><h2>{project.name}</h2>{project.language && <span className="language-tag">{project.language}</span>}{project.is_public ? <span className="visibility-tag" title="Visible on your public profile"><Globe2 size={12} aria-hidden="true" />Public</span> : <span className="visibility-tag" title="Only visible to you"><LockKeyhole size={12} aria-hidden="true" />Private</span>}</div>
                  <p>{project.description || 'No description yet. Let the work show you what this becomes.'}</p>
                  {repository && <a href={repository} target="_blank" rel="noreferrer" className="repository-link">{repository.replace(/^https?:\/\//, '').replace(/\/$/, '')}<ExternalLink size={12} aria-hidden="true" /><span className="sr-only"> (opens in a new tab)</span></a>}
                  <button className="text-button project-visibility-button" type="button" onClick={() => void toggleVisibility(project)} disabled={visibilityPending === project.id} aria-label={`Make ${project.name} ${project.is_public ? 'private' : 'public'}`}>
                    {visibilityPending === project.id ? 'Saving…' : project.is_public ? <><LockKeyhole size={13} aria-hidden="true" />Make private</> : <><Globe2 size={13} aria-hidden="true" />Make public</>}
                  </button>
                </div>
              </div>
              <div className="project-number" data-label="Today"><strong>{formatDuration(project.today_seconds)}</strong></div>
              <div className="project-number" data-label="All time"><strong>{formatDuration(project.total_seconds)}</strong></div>
              <div className="project-last-active" data-label="Last active">{relativeTime(project.last_active_at)}</div>
            </article>
          );
        }) : (
          <EmptyState icon={<FolderPlus size={25} aria-hidden="true" />} title="Give your next idea a home" action={<button className="button button-primary" type="button" onClick={() => setOpen(true)}><Plus size={17} aria-hidden="true" />Create your first project</button>}>
            <p>Projects gather sessions and evidence into a story you can reflect on. Add one now, or let your local agent match one by repository.</p>
          </EmptyState>
        )}
      </section>
      <p className="field-hint visibility-policy-note">Public projects appear only when your profile is public. Visibility changes do not alter reviewer links you already created; those snapshots are immutable.</p>

      <Modal open={open} onClose={() => !pending && setOpen(false)} title="Start a new project" eyebrow="A NEW THREAD" className="form-modal">
        <form className="stack-form" onSubmit={submit}>
          <p className="modal-intro">Name the thing you’re making. Projects are public on your public profile by default; you can change visibility at any time.</p>
          {error && <InlineMessage tone="error">{error}</InlineMessage>}
          <div className="field"><label htmlFor="project-name">Project name</label><input id="project-name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required maxLength={80} placeholder="e.g. Weather radio" /></div>
          <div className="field"><label htmlFor="project-repository">Repository URL <span className="optional">Optional</span></label><input id="project-repository" type="url" value={form.repository_url} onChange={(event) => setForm({ ...form, repository_url: event.target.value })} maxLength={500} placeholder="https://github.com/you/project" inputMode="url" /></div>
          <div className="field"><label htmlFor="project-description">What are you exploring? <span className="optional">Optional</span></label><textarea id="project-description" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} rows={3} maxLength={500} placeholder="A sentence or two is plenty." /></div>
          <div className="field color-field"><label htmlFor="project-color">Thread color</label><div><input id="project-color" type="color" value={form.color} onChange={(event) => setForm({ ...form, color: event.target.value })} /><span>{form.color.toUpperCase()}</span><small>Used to identify this project in charts and timelines.</small></div></div>
          <div className="modal-actions"><button className="button button-secondary" type="button" onClick={() => setOpen(false)} disabled={pending}>Cancel</button><button className="button button-primary" type="submit" disabled={pending}>{pending ? 'Creating…' : 'Create project'}</button></div>
        </form>
      </Modal>
    </div>
  );
}
