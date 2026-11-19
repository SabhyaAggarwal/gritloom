import { useState } from 'react';
import type { ReactNode } from 'react';
import {
  FolderKanban,
  Home,
  LogOut,
  Menu,
  Send,
  Settings2,
  ShieldCheck,
  SquareArrowOutUpRight,
} from 'lucide-react';
import type { ConnectionState, User } from '../types';
import { AppLink, useRouter } from '../router';
import { errorText, initials } from '../utils';
import { InlineMessage, Logo, Modal } from './ui';

const navigation = [
  { to: '/', label: 'Overview', icon: Home },
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/submissions', label: 'Submissions', icon: Send },
  { to: '/settings', label: 'Settings & privacy', icon: Settings2 },
];

function ConnectionNote({ state, active }: { state: ConnectionState; active: boolean }) {
  const label = state === 'live' ? 'Live updates' : state === 'polling' ? '30-second updates' : state === 'connecting' ? 'Connecting…' : state === 'demo' ? 'Sample workspace' : 'Updates offline';
  return (
    <div className="connection-note">
      <span className={`connection-dot connection-${state}`} aria-hidden="true" />
      <div><strong>{label}</strong><span>{active ? 'Local agent active' : state === 'polling' ? 'Live stream unavailable' : 'Agent currently idle'}</span></div>
    </div>
  );
}

function SidebarContent({ user, connection, live, onLogout, onNavigate }: {
  user: User;
  connection: ConnectionState;
  live: boolean;
  onLogout: () => Promise<void>;
  onNavigate?: () => void;
}) {
  const { path } = useRouter();
  const [loggingOut, setLoggingOut] = useState(false);
  const [logoutError, setLogoutError] = useState('');

  const logout = async () => {
    setLoggingOut(true);
    setLogoutError('');
    try {
      await onLogout();
      onNavigate?.();
    } catch (error) {
      setLogoutError(errorText(error));
    } finally {
      setLoggingOut(false);
    }
  };

  return (
    <div className="sidebar-content">
      <AppLink to="/" className="sidebar-brand" onClick={onNavigate}><Logo light /></AppLink>
      <div className="sidebar-caption"><span />YOUR CREATIVE RECORD</div>
      <nav className="main-nav" aria-label="Main navigation">
        {navigation.map(({ to, label, icon: Icon }) => {
          const active = to === '/' ? path === '/' : path.startsWith(to);
          return (
            <AppLink key={to} to={to} onClick={onNavigate} className={active ? 'nav-link nav-active' : 'nav-link'} aria-current={active ? 'page' : undefined}>
              <Icon size={18} strokeWidth={1.8} aria-hidden="true" />
              <span>{label}</span>
              {active && <i aria-hidden="true" />}
            </AppLink>
          );
        })}
      </nav>
      <div className="sidebar-spacer" />
      <div className="sidebar-privacy">
        <ShieldCheck size={19} aria-hidden="true" />
        <div><strong>Private by design</strong><p>Your evidence is yours. You decide what leaves the loom.</p></div>
      </div>
      <ConnectionNote state={connection} active={live} />
      <AppLink to={`/u/${user.username}`} className="public-profile-link" onClick={onNavigate}>
        View public profile <SquareArrowOutUpRight size={14} aria-hidden="true" />
      </AppLink>
      <div className="sidebar-user">
        <AppLink to="/settings" className="user-identity" onClick={onNavigate} aria-label="Open profile settings">
          <span className="avatar avatar-small">{initials(user.display_name)}</span>
          <span><strong>{user.display_name}</strong><small>@{user.username}</small></span>
        </AppLink>
        <button className="logout-button" type="button" onClick={logout} disabled={loggingOut} aria-label={loggingOut ? 'Signing out' : 'Sign out'} title="Sign out">
          <LogOut size={17} aria-hidden="true" />
        </button>
      </div>
      {logoutError && <InlineMessage tone="error">{logoutError}</InlineMessage>}
      <a className="sidebar-license" href="https://github.com/SabhyaAggarwal/gritloom" target="_blank" rel="noreferrer">AGPLv3 <span>·</span> source code</a>
    </div>
  );
}

export function AppShell({ children, user, connection, live, onLogout }: {
  children: ReactNode;
  user: User;
  connection: ConnectionState;
  live: boolean;
  onLogout: () => Promise<void>;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">Skip to main content</a>
      <aside className="sidebar" aria-label="Workspace sidebar">
        <SidebarContent user={user} connection={connection} live={live} onLogout={onLogout} />
      </aside>
      <header className="mobile-header">
        <AppLink to="/" aria-label="GritLoom overview"><Logo /></AppLink>
        <button className="button button-secondary button-small mobile-menu-button" type="button" onClick={() => setMenuOpen(true)} aria-haspopup="dialog" aria-expanded={menuOpen}>
          <Menu size={18} aria-hidden="true" /> Menu
        </button>
      </header>
      <Modal open={menuOpen} onClose={() => setMenuOpen(false)} title="Navigate GritLoom" className="mobile-nav-modal">
        <SidebarContent user={user} connection={connection} live={live} onLogout={onLogout} onNavigate={() => setMenuOpen(false)} />
      </Modal>
      <main className="main-content" id="main-content" tabIndex={-1}>{children}</main>
    </div>
  );
}
