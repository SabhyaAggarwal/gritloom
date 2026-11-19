import { ArrowLeft } from 'lucide-react';
import { AppLink } from '../router';
import { Logo } from '../components/ui';

export function NotFound() {
  return (
    <main className="not-found-page">
      <Logo />
      <div><p className="eyebrow">A LOOSE THREAD</p><h1>This page isn’t on the loom.</h1><p>The address may have changed, or the link may be incomplete.</p><AppLink to="/" className="button button-primary"><ArrowLeft size={17} aria-hidden="true" />Back to GritLoom</AppLink></div>
    </main>
  );
}
