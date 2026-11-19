import type { Dashboard, Device, Project, Session, Submission, User } from './types';
import { localDateKey } from './utils';

const now = () => Date.now();
const iso = (milliseconds: number) => new Date(milliseconds).toISOString();

export const demoUser: User = {
  id: 'demo-user',
  username: 'maya',
  display_name: 'Maya Chen',
  bio: 'Seventeen, curious, and building small tools for a more understandable internet. Currently learning systems design through making.',
  is_public: true,
  timezone: 'America/Los_Angeles',
  created_at: '2025-09-14T15:00:00Z',
};

const projectSeed: Array<Omit<Project, 'last_active_at'>> = [
  {
    id: 'project-canopy', name: 'Canopy', slug: 'canopy',
    repository_url: 'https://github.com/example/canopy',
    description: 'A calm, keyboard-first reading list that turns saved links into a thoughtful weekly digest.',
    color: '#2f6a4f', language: 'TypeScript', total_seconds: 102300, today_seconds: 7200, is_public: true,
  },
  {
    id: 'project-loombox', name: 'Loombox', slug: 'loombox',
    repository_url: 'https://github.com/example/loombox',
    description: 'A tiny hardware synth built with an RP2040, reclaimed buttons, and too much curiosity.',
    color: '#df755c', language: 'C++', total_seconds: 65700, today_seconds: 3600, is_public: true,
  },
  {
    id: 'project-fieldnotes', name: 'Fieldnotes', slug: 'fieldnotes',
    repository_url: 'https://github.com/example/fieldnotes',
    description: 'Offline-first nature observations for a school biodiversity project.',
    color: '#c49a45', language: 'Svelte', total_seconds: 41400, today_seconds: 0, is_public: true,
  },
  {
    id: 'project-ideas', name: 'Idea sketches', slug: 'idea-sketches',
    repository_url: '', description: 'Early experiments and unpolished prototypes. Not ready to share yet.',
    color: '#738078', language: 'Python', total_seconds: 19800, today_seconds: 1200, is_public: false,
  },
];

function session(
  id: string,
  project: Project,
  start: number,
  seconds: number,
  details: Pick<Session, 'kind' | 'source' | 'verification' | 'edit_count' | 'ai_count' | 'test_count' | 'failed_test_count' | 'commit_count'>,
): Session {
  return {
    id,
    project_id: project.id,
    project_name: project.name,
    project_color: project.color,
    start_at: iso(start),
    end_at: iso(start + seconds * 1000),
    seconds,
    ...details,
  };
}

export function createDemoDashboard(): Dashboard {
  const timestamp = now();
  const projects = projectSeed.map((project, index) => ({
    ...project,
    last_active_at: iso(timestamp - [20, 110, 2900, 480][index] * 60000),
  }));
  const [canopy, loombox, fieldnotes, ideas] = projects;
  const currentHour = new Date().getHours();
  const liveStart = timestamp - 22 * 60000;
  const timeline = [
    session('session-live', canopy, liveStart, 1320, {
      kind: 'engaged', source: 'VS Code · local agent', verification: 'observed',
      edit_count: 18, ai_count: 3, test_count: 7, failed_test_count: 1, commit_count: 0,
    }),
    session('session-ai', canopy, timestamp - 78 * 60000, 1800, {
      kind: 'background_ai', source: 'Claude Code · local agent', verification: 'corroborated',
      edit_count: 7, ai_count: 12, test_count: 14, failed_test_count: 2, commit_count: 1,
    }),
    session('session-review', canopy, timestamp - 156 * 60000, 3600, {
      kind: 'engaged', source: 'VS Code + terminal', verification: 'review_ready',
      edit_count: 64, ai_count: 8, test_count: 21, failed_test_count: 3, commit_count: 2,
    }),
    session('session-hardware', loombox, timestamp - 27 * 3600000, 4080, {
      kind: 'engaged', source: 'Neovim · local agent', verification: 'corroborated',
      edit_count: 42, ai_count: 2, test_count: 8, failed_test_count: 4, commit_count: 1,
    }),
    session('session-field', fieldnotes, timestamp - 51 * 3600000, 3060, {
      kind: 'engaged', source: 'Zed · local agent', verification: 'observed',
      edit_count: 31, ai_count: 0, test_count: 5, failed_test_count: 0, commit_count: 1,
    }),
    session('session-private', ideas, timestamp - 76 * 3600000, 2400, {
      kind: 'engaged', source: 'VS Code · local agent', verification: 'observed',
      edit_count: 22, ai_count: 5, test_count: 3, failed_test_count: 1, commit_count: 0,
    }),
  ];
  // Keep the sample believable even if the demo is opened very early in the morning.
  if (currentHour < 4) {
    timeline.forEach((item, index) => {
      item.start_at = iso(timestamp - (index + 1) * 3600000);
      item.end_at = iso(new Date(item.start_at).getTime() + item.seconds * 1000);
    });
  }

  const seconds = [5400, 8400, 3600, 10800, 6600, 9600, 11400];
  const aiSeconds = [600, 1200, 0, 1800, 900, 1200, 1800];
  const daily = seconds.map((value, index) => {
    const date = new Date(timestamp);
    date.setDate(date.getDate() - (6 - index));
    return { date: localDateKey(date), seconds: value, ai_seconds: aiSeconds[index] };
  });

  return {
    user: { ...demoUser },
    stats: {
      today_seconds: 11400,
      week_seconds: daily.reduce((sum, day) => sum + day.seconds, 0),
      total_seconds: 229200,
      streak_days: 12,
      weekly_goal_seconds: 72000,
      ai_seconds: 7500,
      background_ai_seconds: 5400,
      iteration_count: 183,
    },
    projects,
    daily,
    timeline,
    live: true,
  };
}

export function createDemoSubmission(token = 'woven-evidence-2026'): Submission {
  const dashboard = createDemoDashboard();
  const sessions = dashboard.timeline.filter((item) => ['session-review', 'session-live', 'session-hardware'].includes(item.id)).map((item, sessionIndex) => {
    const occurred = new Date(item.start_at).getTime();
    const file = item.project_id === 'project-loombox' ? 'src/synth.cpp' : 'src/parser.ts';
    return {
      ...item,
      evidence_hash: Array.from({ length: 64 }, (_, index) => ((index + sessionIndex * 3) % 16).toString(16)).join(''),
      evidence: [
        { id: `${item.id}-edit`, occurred_at: iso(occurred + 120000), source: 'vscode', kind: 'edit', file, additions: 28 + sessionIndex * 7, deletions: 9 + sessionIndex * 2, ai: false, background: false },
        { id: `${item.id}-ai`, occurred_at: iso(occurred + 240000), source: 'claude-code', kind: 'ai_tool', additions: 0, deletions: 0, ai: true, background: false, model: 'local-sample' },
        { id: `${item.id}-test`, occurred_at: iso(occurred + 420000), source: 'terminal', kind: 'test', additions: 0, deletions: 0, exit_code: sessionIndex === 1 ? 1 : 0, ai: false, background: false },
        { id: `${item.id}-commit`, occurred_at: iso(occurred + 600000), source: 'git', kind: 'commit', additions: 0, deletions: 0, ai: false, background: false, commit_hash: `a${sessionIndex}8f42c9d13b7e`, branch: 'main' },
      ],
    };
  });
  return {
    id: 'submission-demo',
    author: demoUser.username,
    token: token || undefined,
    created_at: iso(now() - 26 * 60000),
    note: 'This is the reading-list parser milestone. I started with a brittle regex, then used tests to understand the edge cases before asking AI to suggest a tokenizer. The failed tests are included because they show where my mental model changed. I would especially like feedback on the parser boundaries and test names.',
    seconds: sessions.reduce((sum, item) => sum + item.seconds, 0),
    session_count: sessions.length,
    status: 'ready for review',
    manifest_hash: '9f2b41d7a38e0d6351e5aa063d802e72d1f0a83ab49aef33efab27c74a6f2e91c0',
    sessions,
  };
}

export const demoDevices: Device[] = [
  { id: 'device-laptop', name: 'Framework laptop · VS Code', created_at: '2026-10-28T14:30:00Z' },
  { id: 'device-desk', name: 'Desk terminal', created_at: '2026-11-12T18:10:00Z' },
];
