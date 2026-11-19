import { useState } from 'react';
import type { FormEvent } from 'react';
import { ArrowRight, Braces, LockKeyhole, Sprout } from 'lucide-react';
import { api } from '../api';
import { AppLink } from '../router';
import type { User } from '../types';
import { errorText } from '../utils';
import { InlineMessage, Logo } from '../components/ui';

export function AuthView({ mode, onAuthenticated }: { mode: 'login' | 'register'; onAuthenticated: (user: User) => void }) {
  const register = mode === 'register';
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setPending(true);
    setError('');
    try {
      const user = register
        ? await api.register(username.trim(), displayName.trim(), password)
        : await api.login(username.trim(), password);
      onAuthenticated(user);
    } catch (reason) {
      setError(errorText(reason));
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="auth-page">
      <a className="skip-link" href="#auth-form">Skip to sign in form</a>
      <header className="auth-header">
        <AppLink to="/login" aria-label="GritLoom sign in"><Logo /></AppLink>
        <span>AGPLv3 · privacy-friendly by default</span>
      </header>
      <main className="auth-layout">
        <section className="auth-story" aria-labelledby="auth-story-title">
          <p className="eyebrow"><span className="eyebrow-line" />A QUIETER KIND OF TRACKER</p>
          <h1 id="auth-story-title">Your effort tells a story.<br /><em>Keep the pen.</em></h1>
          <p className="auth-intro">GritLoom weaves focused work, iterations, and evidence into a record you can learn from. Not a scoreboard. Not someone else’s dashboard.</p>
          <ul className="auth-principles">
            <li><Sprout size={20} aria-hidden="true" /><div><strong>Built for reflection</strong><span>Notice momentum, practice, and growth—not “productivity” theater.</span></div></li>
            <li><Braces size={20} aria-hidden="true" /><div><strong>AI has its own thread</strong><span>Background AI time is labeled separately from your engaged making.</span></div></li>
            <li><LockKeyhole size={20} aria-hidden="true" /><div><strong>Sharing is deliberate</strong><span>Your profile is public by default and can be made private. Reviewer links contain only sessions you select.</span></div></li>
          </ul>
          <div className="loom-sketch" aria-hidden="true">
            <span /><span /><span /><span /><span /><i /><i /><b>MAKE · NOTICE · ITERATE</b>
          </div>
        </section>
        <section className="auth-form-section" id="auth-form" aria-labelledby="auth-title">
          <div className="auth-form-heading">
            <p className="eyebrow">{register ? 'START YOUR RECORD' : 'BACK TO THE WORKBENCH'}</p>
            <h2 id="auth-title">{register ? 'Create your GritLoom' : 'Welcome back, builder.'}</h2>
            <p>{register ? 'A private home for the work you are already doing.' : 'Pick up the thread. Your workspace is waiting.'}</p>
          </div>
          <form className="stack-form" onSubmit={submit}>
            {error && <InlineMessage tone="error">{error}</InlineMessage>}
            {register && (
              <div className="field">
                <label htmlFor="display-name">Display name</label>
                <input id="display-name" name="display_name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} autoComplete="name" required maxLength={80} placeholder="How you’d like to be known" />
              </div>
            )}
            <div className="field">
              <label htmlFor="username">Username</label>
              <input id="username" name="username" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" autoCapitalize="none" spellCheck={false} required minLength={register ? 3 : undefined} maxLength={register ? 24 : 64} pattern={register ? '[A-Za-z0-9][A-Za-z0-9_\\-]{2,23}' : undefined} title={register ? '3–24 letters, numbers, underscores, or hyphens' : undefined} placeholder={register ? 'your-public-handle' : 'Your username'} />
              {register && <p className="field-hint">Used for your public profile if you choose to enable it.</p>}
            </div>
            <div className="field">
              <label htmlFor="password">Password</label>
              <input id="password" name="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete={register ? 'new-password' : 'current-password'} required minLength={register ? 12 : undefined} maxLength={128} placeholder={register ? 'At least 12 characters' : 'Your password'} />
            </div>
            <button className="button button-primary button-wide" type="submit" disabled={pending}>
              {pending ? <span className="button-spinner" aria-hidden="true" /> : <ArrowRight size={18} aria-hidden="true" />}
              {pending ? (register ? 'Creating your workspace…' : 'Signing in…') : (register ? 'Create your workspace' : 'Sign in')}
            </button>
          </form>
          <p className="auth-switch">
            {register ? 'Already have a workspace?' : 'New to GritLoom?'}{' '}
            <AppLink to={register ? '/login' : '/register'} className="text-link">{register ? 'Sign in' : 'Create an account'}</AppLink>
          </p>
          <div className="demo-invitation">
            <div><strong>Just looking around?</strong><p>Open an explicit sample workspace. No sign-in and no API data.</p></div>
            <a className="button button-secondary button-small" href="/?demo=1">Explore demo <ArrowRight size={15} aria-hidden="true" /></a>
          </div>
        </section>
      </main>
      <footer className="auth-footer"><span>Time in service of self-improvement.</span><span>Your data. Your context. Your call. · <a href="https://github.com/SabhyaAggarwal/gritloom" target="_blank" rel="noreferrer">AGPLv3 source</a></span></footer>
    </div>
  );
}
