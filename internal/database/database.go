// Package database opens and migrates the GritLoom SQLite database.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &DB{db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE,
    display_name TEXT NOT NULL,
    bio TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    is_public INTEGER NOT NULL DEFAULT 1,
    weekly_goal_seconds INTEGER NOT NULL DEFAULT 36000,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS auth_sessions_user ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS auth_sessions_expiry ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER,
    revoked_at INTEGER
) STRICT;
CREATE INDEX IF NOT EXISTS devices_user ON devices(user_id);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_key TEXT NOT NULL,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    repository_url TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '#287a5a',
    language TEXT NOT NULL DEFAULT '',
    is_public INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    last_active_at INTEGER,
    UNIQUE(user_id, project_key)
) STRICT;
CREATE INDEX IF NOT EXISTS projects_user ON projects(user_id, last_active_at DESC);

CREATE TABLE IF NOT EXISTS events (
    id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    occurred_at INTEGER NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    kind TEXT NOT NULL,
    file TEXT NOT NULL DEFAULT '',
    additions INTEGER NOT NULL DEFAULT 0,
    deletions INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER,
    ai INTEGER NOT NULL DEFAULT 0,
    background INTEGER NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    commit_hash TEXT NOT NULL DEFAULT '',
    fingerprint TEXT NOT NULL,
    PRIMARY KEY(user_id, id)
) STRICT;
CREATE INDEX IF NOT EXISTS events_user_time ON events(user_id, occurred_at);
CREATE INDEX IF NOT EXISTS events_project_time ON events(project_id, occurred_at);

CREATE TABLE IF NOT EXISTS work_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    start_at INTEGER NOT NULL,
    end_at INTEGER NOT NULL,
    seconds INTEGER NOT NULL,
    kind TEXT NOT NULL,
    source TEXT NOT NULL,
    verification TEXT NOT NULL,
    edit_count INTEGER NOT NULL DEFAULT 0,
    ai_count INTEGER NOT NULL DEFAULT 0,
    test_count INTEGER NOT NULL DEFAULT 0,
    failed_test_count INTEGER NOT NULL DEFAULT 0,
    commit_count INTEGER NOT NULL DEFAULT 0,
    evidence_hash TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '[]'
) STRICT;
CREATE INDEX IF NOT EXISTS work_sessions_user_time ON work_sessions(user_id, start_at DESC);
CREATE INDEX IF NOT EXISTS work_sessions_project ON work_sessions(project_id, start_at DESC);

CREATE TABLE IF NOT EXISTS submissions (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note TEXT NOT NULL DEFAULT '',
    seconds INTEGER NOT NULL,
    session_count INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    manifest_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS submissions_user ON submissions(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS submission_sessions (
    submission_id TEXT NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    snapshot TEXT NOT NULL,
    PRIMARY KEY(submission_id, session_id)
) STRICT;
`
