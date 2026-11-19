import { ArrowUpRight } from 'lucide-react';
import { AppLink } from '../router';
import { Logo } from './ui';

export function PublicHeader() {
  return (
    <header className="public-header">
      <AppLink to="/" aria-label="GritLoom home"><Logo /></AppLink>
      <span className="public-header-note">A record of learning, not a score. · <a href="https://github.com/SabhyaAggarwal/gritloom" target="_blank" rel="noreferrer">AGPLv3 source</a></span>
      <AppLink to="/login" className="text-link">Open GritLoom <ArrowUpRight size={15} aria-hidden="true" /></AppLink>
    </header>
  );
}
