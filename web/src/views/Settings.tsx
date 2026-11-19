import { useCallback, useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { ArrowUpRight, Check, Eye, Globe2, KeyRound, Laptop, LockKeyhole, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { api } from '../api';
import { demoDevices } from '../demo';
import { AppLink } from '../router';
import type { Device, DeviceWithToken, ProfileUpdate, User } from '../types';
import { errorText, formatDate } from '../utils';
import { CopyButton, EmptyState, ErrorState, InlineMessage, LoadingBlock, Modal } from '../components/ui';

export function Settings({ user, weeklyGoalSeconds, demo, onSave }: {
  user: User;
  weeklyGoalSeconds: number;
  demo: boolean;
  onSave: (input: ProfileUpdate) => Promise<void>;
}) {
  const browserTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  const timeZones = useMemo(() => {
    const intl = Intl as typeof Intl & { supportedValuesOf?: (key: 'timeZone') => string[] };
    const values = intl.supportedValuesOf?.('timeZone') ?? ['UTC', 'America/New_York', 'America/Los_Angeles', 'Europe/London', 'Europe/Berlin', 'Asia/Kolkata', 'Asia/Tokyo', 'Australia/Sydney'];
    return [...new Set([browserTimeZone, user.timezone || 'UTC', ...values])].sort();
  }, [browserTimeZone, user.timezone]);
  const [displayName, setDisplayName] = useState(user.display_name);
  const [bio, setBio] = useState(user.bio);
  const [isPublic, setIsPublic] = useState(user.is_public);
  const [goalHours, setGoalHours] = useState(weeklyGoalSeconds / 3600);
  const [timezone, setTimezone] = useState(user.timezone || browserTimeZone);
  const [profilePending, setProfilePending] = useState(false);
  const [profileError, setProfileError] = useState('');
  const [profileSaved, setProfileSaved] = useState(false);
  const [devices, setDevices] = useState<Device[]>(() => demo ? [...demoDevices] : []);
  const [devicesLoading, setDevicesLoading] = useState(!demo);
  const [devicesError, setDevicesError] = useState('');
  const [deviceName, setDeviceName] = useState('');
  const [devicePending, setDevicePending] = useState(false);
  const [deviceError, setDeviceError] = useState('');
  const [newToken, setNewToken] = useState<DeviceWithToken | null>(null);
  const [tokenSaved, setTokenSaved] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<Device | null>(null);
  const [revokePending, setRevokePending] = useState(false);
  const [revokeError, setRevokeError] = useState('');

  useEffect(() => {
    setDisplayName(user.display_name);
    setBio(user.bio);
    setIsPublic(user.is_public);
    setTimezone(user.timezone || browserTimeZone);
  }, [user, browserTimeZone]);
  useEffect(() => setGoalHours(weeklyGoalSeconds / 3600), [weeklyGoalSeconds]);

  const loadDevices = useCallback(async () => {
    if (demo) {
      setDevicesLoading(false);
      return;
    }
    setDevicesLoading(true);
    setDevicesError('');
    try {
      setDevices(await api.devices());
    } catch (error) {
      setDevicesError(errorText(error));
    } finally {
      setDevicesLoading(false);
    }
  }, [demo]);

  useEffect(() => {
    void loadDevices();
  }, [loadDevices]);

  const saveProfile = async (event: FormEvent) => {
    event.preventDefault();
    setProfilePending(true);
    setProfileError('');
    setProfileSaved(false);
    try {
      await onSave({ display_name: displayName.trim(), bio: bio.trim(), is_public: isPublic, weekly_goal_seconds: Math.round(goalHours * 3600), timezone });
      setProfileSaved(true);
    } catch (error) {
      setProfileError(errorText(error));
    } finally {
      setProfilePending(false);
    }
  };

  const createDevice = async (event: FormEvent) => {
    event.preventDefault();
    setDevicePending(true);
    setDeviceError('');
    try {
      const created = demo ? {
        id: `demo-device-${Date.now()}`,
        name: deviceName.trim(),
        token: `gl_demo_${Math.random().toString(36).slice(2)}${Math.random().toString(36).slice(2)}`,
        created_at: new Date().toISOString(),
      } : await api.createDevice(deviceName.trim());
      setDevices((current) => [...current, { id: created.id, name: created.name, created_at: created.created_at }]);
      setNewToken(created);
      setTokenSaved(false);
      setDeviceName('');
    } catch (error) {
      setDeviceError(errorText(error));
    } finally {
      setDevicePending(false);
    }
  };

  const revoke = async () => {
    if (!revokeTarget) return;
    setRevokePending(true);
    setRevokeError('');
    try {
      if (!demo) await api.deleteDevice(revokeTarget.id);
      setDevices((current) => current.filter((device) => device.id !== revokeTarget.id));
      setRevokeTarget(null);
    } catch (error) {
      setRevokeError(errorText(error));
    } finally {
      setRevokePending(false);
    }
  };

  const profileUrl = `${window.location.origin}/u/${encodeURIComponent(user.username)}${demo ? '?demo=1' : ''}`;

  return (
    <div className="page settings-page">
      <header className="page-heading">
        <div><p className="eyebrow">YOUR RECORD, YOUR RULES</p><h1>Settings & privacy</h1><p>Choose how you appear, what you share, and which devices can add to your record.</p></div>
        <span className="heading-note"><ShieldCheck size={18} aria-hidden="true" />Public by default · private anytime</span>
      </header>

      <div className="settings-layout">
        <div className="settings-main">
          <section className="settings-section" aria-labelledby="profile-settings-title">
            <header className="settings-section-header"><div><span className="section-number">01</span><h2 id="profile-settings-title">Profile & rhythm</h2></div><p>These choices shape your workspace and optional public page.</p></header>
            <form className="stack-form" onSubmit={saveProfile}>
              {profileError && <InlineMessage tone="error">{profileError}</InlineMessage>}
              {profileSaved && <InlineMessage tone="success">Your settings are saved.</InlineMessage>}
              <div className="field"><label htmlFor="settings-display-name">Display name</label><input id="settings-display-name" value={displayName} onChange={(event) => { setDisplayName(event.target.value); setProfileSaved(false); }} required maxLength={80} autoComplete="name" /></div>
              <div className="field"><label htmlFor="settings-bio">Bio <span className="optional">Optional</span></label><textarea id="settings-bio" value={bio} onChange={(event) => { setBio(event.target.value); setProfileSaved(false); }} rows={4} maxLength={500} placeholder="What are you curious about? What are you learning by making?" /><p className="field-hint character-count">{bio.length} / 500</p></div>
              <div className="field"><label htmlFor="settings-goal">Weekly engaged goal</label><div className="input-suffix"><input id="settings-goal" type="number" min={0.5} max={168} step={0.25} value={goalHours} onChange={(event) => { setGoalHours(Number(event.target.value)); setProfileSaved(false); }} required /><span>hours / week</span></div><p className="field-hint">A reflection aid, not a quota. Background AI is never counted toward this goal.</p></div>
              <div className="field"><label htmlFor="settings-timezone">Timezone</label><select id="settings-timezone" value={timezone} onChange={(event) => { setTimezone(event.target.value); setProfileSaved(false); }} required>{timeZones.map((zone) => <option key={zone} value={zone}>{zone.replace(/_/g, ' ')}</option>)}</select><p className="field-hint">An IANA timezone used for your daily boundaries, weekly rhythm, and streaks.</p></div>
              <fieldset className="visibility-control">
                <legend>Profile visibility</legend>
                <label className={`visibility-option ${!isPublic ? 'visibility-selected' : ''}`}><input type="radio" name="visibility" checked={!isPublic} onChange={() => { setIsPublic(false); setProfileSaved(false); }} /><LockKeyhole size={20} aria-hidden="true" /><span><strong>Private</strong><small>Only you can open your dashboard and profile.</small></span>{!isPublic && <Check size={18} aria-hidden="true" />}</label>
                <label className={`visibility-option ${isPublic ? 'visibility-selected' : ''}`}><input type="radio" name="visibility" checked={isPublic} onChange={() => { setIsPublic(true); setProfileSaved(false); }} /><Globe2 size={20} aria-hidden="true" /><span><strong>Public</strong><small>Your public profile can show your bio, public projects, and shared stats.</small></span>{isPublic && <Check size={18} aria-hidden="true" />}</label>
              </fieldset>
              <div className="form-footer"><p>Changing visibility never creates a reviewer submission. Those are always selected separately.</p><button className="button button-primary" type="submit" disabled={profilePending}>{profilePending ? 'Saving…' : 'Save settings'}</button></div>
            </form>
          </section>

          <section className="settings-section" aria-labelledby="public-link-title">
            <header className="settings-section-header"><div><span className="section-number">02</span><h2 id="public-link-title">Your public address</h2></div><p>A single page for the work you intentionally make public.</p></header>
            <div className="public-address"><Globe2 size={20} aria-hidden="true" /><div><strong>gritloom.local/u/{user.username}</strong><p>{isPublic ? 'Your public profile is enabled.' : 'Enable a public profile above before sharing this address.'}</p></div></div>
            <div className="field"><label htmlFor="public-profile-url">Profile URL</label><div className="copy-field"><input id="public-profile-url" value={profileUrl} readOnly onFocus={(event) => event.target.select()} /><CopyButton value={profileUrl} label="Copy URL" /></div></div>
            {isPublic && <AppLink to={`/u/${user.username}`} className="text-link"><Eye size={15} aria-hidden="true" />Preview public profile <ArrowUpRight size={14} aria-hidden="true" /></AppLink>}
          </section>

          <section className="settings-section" id="devices" aria-labelledby="devices-title">
            <header className="settings-section-header"><div><span className="section-number">03</span><h2 id="devices-title">Devices & agent access</h2></div><p>Each device gets its own revocable token. Tokens are never listed again.</p></header>
            <form className="device-create-form" onSubmit={createDevice}>
              <div className="field"><label htmlFor="device-name">Connect a new device</label><input id="device-name" value={deviceName} onChange={(event) => setDeviceName(event.target.value)} required maxLength={80} placeholder="e.g. Framework laptop · VS Code" /><p className="field-hint">Use a name you’ll recognize if you need to revoke access later.</p></div>
              <button className="button button-secondary" type="submit" disabled={devicePending}><Plus size={17} aria-hidden="true" />{devicePending ? 'Generating…' : 'Generate device token'}</button>
            </form>
            {deviceError && <InlineMessage tone="error">{deviceError}</InlineMessage>}
            <div className="device-list-wrap">
              {devicesLoading ? <LoadingBlock label="Loading connected devices…" /> : devicesError ? <ErrorState message={devicesError} onRetry={() => void loadDevices()} /> : devices.length ? <ul className="device-list">{devices.map((device) => <li key={device.id}><span className="device-icon"><Laptop size={20} aria-hidden="true" /></span><div><strong>{device.name}</strong><small>Connected {formatDate(device.created_at)}</small></div><span className="device-active"><i aria-hidden="true" />Authorized</span><button className="button button-danger-quiet button-small" type="button" onClick={() => { setRevokeTarget(device); setRevokeError(''); }}><Trash2 size={15} aria-hidden="true" />Revoke</button></li>)}</ul> : <EmptyState icon={<KeyRound size={24} aria-hidden="true" />} title="No connected devices"><p>Generate a token above to authorize the local GritLoom agent on your first device.</p></EmptyState>}
            </div>
          </section>
        </div>

        <aside className="settings-aside">
          <section className="privacy-manifesto"><ShieldCheck size={26} aria-hidden="true" /><p className="eyebrow">THE GRITLOOM PROMISE</p><h2>Insight without the watching.</h2><ul><li><Check size={16} aria-hidden="true" />Engaged and AI time stay distinct.</li><li><Check size={16} aria-hidden="true" />Public profiles contain public data only.</li><li><Check size={16} aria-hidden="true" />Reviewer links include selected sessions only.</li><li><Check size={16} aria-hidden="true" />Device access can be revoked at any time.</li></ul><p>GritLoom is AGPLv3 software. You can inspect the code and understand how your record is made.</p></section>
          <section className="settings-help"><KeyRound size={20} aria-hidden="true" /><h3>What a device token does</h3><p>It lets one local agent add sessions to your account. It does not belong in a repository, screenshot, or reviewer submission.</p></section>
        </aside>
      </div>

      <Modal open={Boolean(newToken)} onClose={() => setNewToken(null)} title="Save your device token" eyebrow="SHOWN ONLY ONCE" className="token-modal">
        {newToken && <div className="stack-form">
          <div className="token-heading-icon"><KeyRound size={28} aria-hidden="true" /></div>
          <p className="modal-intro"><strong>{newToken.name}</strong> is ready to connect. Copy this token into the local GritLoom agent’s device setup. For security, it cannot be shown again after you close this dialog.</p>
          <div className="field"><label htmlFor="device-token">Device token</label><textarea id="device-token" className="token-value" value={newToken.token} readOnly rows={3} onFocus={(event) => event.target.select()} /><p className="field-hint">Treat this like a password. Do not commit it or include it in a screenshot.</p></div>
          <CopyButton value={newToken.token} label="Copy device token" copiedLabel="Token copied" />
          <label className="acknowledge-control"><input type="checkbox" checked={tokenSaved} onChange={(event) => setTokenSaved(event.target.checked)} /><span>I’ve stored this token securely and understand it won’t be shown again.</span></label>
          <div className="modal-actions"><button className="button button-primary" type="button" onClick={() => setNewToken(null)} disabled={!tokenSaved}>Done connecting</button></div>
        </div>}
      </Modal>

      <Modal open={Boolean(revokeTarget)} onClose={() => !revokePending && setRevokeTarget(null)} title="Revoke device access?" eyebrow="DEVICE SECURITY" className="confirm-modal">
        {revokeTarget && <div className="stack-form"><p className="modal-intro"><strong>{revokeTarget.name}</strong> will no longer be able to add sessions. Existing evidence stays in your private record.</p>{revokeError && <InlineMessage tone="error">{revokeError}</InlineMessage>}<div className="modal-actions"><button className="button button-secondary" type="button" onClick={() => setRevokeTarget(null)} disabled={revokePending}>Keep device</button><button className="button button-danger" type="button" onClick={() => void revoke()} disabled={revokePending}>{revokePending ? 'Revoking…' : 'Revoke access'}</button></div></div>}
      </Modal>
    </div>
  );
}
