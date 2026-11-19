export interface User {
  id: string;
  username: string;
  display_name: string;
  bio: string;
  is_public: boolean;
  timezone: string;
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  repository_url: string;
  description: string;
  color: string;
  language: string;
  total_seconds: number;
  today_seconds: number;
  last_active_at: string | null;
  is_public: boolean;
}

export type SessionKind = 'engaged' | 'background_ai';
export type Verification = 'observed' | 'corroborated' | 'review_ready';

export interface EvidenceEvent {
  id: string;
  occurred_at: string;
  source: string;
  kind: string;
  file?: string;
  additions: number;
  deletions: number;
  exit_code?: number;
  ai: boolean;
  background: boolean;
  model?: string;
  commit_hash?: string;
  branch?: string;
}

export interface Session {
  id: string;
  project_id: string;
  project_name: string;
  project_color: string;
  start_at: string;
  end_at: string;
  seconds: number;
  kind: SessionKind;
  source: string;
  verification: Verification;
  edit_count: number;
  ai_count: number;
  test_count: number;
  failed_test_count: number;
  commit_count: number;
  evidence_hash?: string;
  evidence?: EvidenceEvent[];
}

export interface DashboardStats {
  today_seconds: number;
  week_seconds: number;
  total_seconds: number;
  streak_days: number;
  weekly_goal_seconds: number;
  ai_seconds: number;
  background_ai_seconds: number;
  iteration_count: number;
}

export interface DailyActivity {
  date: string;
  seconds: number;
  ai_seconds: number;
}

export interface Dashboard {
  user: User;
  stats: DashboardStats;
  projects: Project[];
  daily: DailyActivity[];
  timeline: Session[];
  live: boolean;
}

export interface Device {
  id: string;
  name: string;
  created_at: string;
}

export interface DeviceWithToken extends Device {
  token: string;
}

export interface Submission {
  id: string;
  author: string;
  token?: string;
  created_at: string;
  note: string;
  seconds: number;
  session_count: number;
  status: string;
  manifest_hash: string;
  sessions: Session[];
}

export interface ProfileUpdate {
  display_name: string;
  bio: string;
  is_public: boolean;
  weekly_goal_seconds: number;
  timezone: string;
}

export interface ProjectInput {
  name: string;
  repository_url: string;
  description: string;
  color: string;
}

export type ConnectionState = 'idle' | 'connecting' | 'live' | 'polling' | 'demo';
