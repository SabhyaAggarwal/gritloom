import { useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { AlertCircle, Check, Clipboard, Inbox, LoaderCircle, X } from 'lucide-react';
import { copyText } from '../utils';

export function Logo({ compact = false, light = false }: { compact?: boolean; light?: boolean }) {
  return (
    <span className={`brand ${light ? 'brand-light' : ''}`} aria-label="GritLoom">
      <svg className="brand-mark" viewBox="0 0 40 40" aria-hidden="true">
        <path d="M8 5v30M20 5v30M32 5v30" className="brand-warp" />
        <path d="M4 13h32M4 28h32" className="brand-weft" />
      </svg>
      {!compact && <span className="brand-word">GritLoom<span className="brand-period">.</span></span>}
    </span>
  );
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return <span className="spinner" role="status"><LoaderCircle size={17} aria-hidden="true" /><span>{label}</span></span>;
}

export function LoadingBlock({ label = 'Gathering your workspace…' }: { label?: string }) {
  return (
    <div className="loading-block" role="status" aria-live="polite">
      <div className="skeleton skeleton-heading" />
      <div className="skeleton skeleton-line" />
      <div className="skeleton skeleton-panel" />
      <span className="sr-only">{label}</span>
    </div>
  );
}

export function ErrorState({ message, onRetry, title = 'We couldn’t load this' }: { message: string; onRetry?: () => void; title?: string }) {
  return (
    <div className="state-panel error-state" role="alert">
      <AlertCircle size={24} aria-hidden="true" />
      <div>
        <h2>{title}</h2>
        <p>{message}</p>
        {onRetry && <button className="button button-secondary button-small" type="button" onClick={onRetry}>Try again</button>}
      </div>
    </div>
  );
}

export function EmptyState({ icon, title, children, action }: { icon?: ReactNode; title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <div className="empty-state">
      <div className="empty-icon">{icon ?? <Inbox size={24} aria-hidden="true" />}</div>
      <h3>{title}</h3>
      <div className="empty-copy">{children}</div>
      {action && <div className="empty-action">{action}</div>}
    </div>
  );
}

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  eyebrow?: string;
  children: ReactNode;
  className?: string;
}

export function Modal({ open, onClose, title, eyebrow, children, className = '' }: ModalProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={`modal ${className}`}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="modal-inner">
        <header className="modal-header">
          <div>
            {eyebrow && <p className="eyebrow">{eyebrow}</p>}
            <h2 id={titleId}>{title}</h2>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close dialog" autoFocus>
            <X size={20} aria-hidden="true" />
          </button>
        </header>
        {children}
      </div>
    </dialog>
  );
}

export function CopyButton({ value, label = 'Copy', copiedLabel = 'Copied', className = '' }: { value: string; label?: string; copiedLabel?: string; className?: string }) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle');

  useEffect(() => {
    if (state === 'idle') return;
    const timeout = window.setTimeout(() => setState('idle'), 2500);
    return () => window.clearTimeout(timeout);
  }, [state]);

  return (
    <span className="copy-control">
      <button
        className={`button button-secondary button-small ${className}`}
        type="button"
        onClick={async () => setState(await copyText(value) ? 'copied' : 'failed')}
      >
        {state === 'copied' ? <Check size={15} aria-hidden="true" /> : <Clipboard size={15} aria-hidden="true" />}
        {state === 'copied' ? copiedLabel : label}
      </button>
      <span className="sr-only" role="status" aria-live="polite">
        {state === 'copied' ? 'Copied to clipboard.' : state === 'failed' ? 'Could not copy automatically. Select the text and copy it manually.' : ''}
      </span>
      {state === 'failed' && <span className="field-hint error-text">Select the text and copy it manually.</span>}
    </span>
  );
}

export function InlineMessage({ tone, children }: { tone: 'error' | 'success' | 'info'; children: ReactNode }) {
  return (
    <div className={`inline-message inline-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      {tone === 'success' && <Check size={17} aria-hidden="true" />}
      {tone === 'error' && <AlertCircle size={17} aria-hidden="true" />}
      <div>{children}</div>
    </div>
  );
}
