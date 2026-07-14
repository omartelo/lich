// Package store is lich's persistence layer: a single SQLite database holding
// open projects, their terminal sessions and backend-read settings (currently
// the Claude Code binary path, global or per-project). It never stores chat or
// terminal content — only the metadata needed to restore the workspace after a
// restart.
//
// UI-only preferences (font, theme, zoom) intentionally stay in the frontend's
// localStorage: they need synchronous access on first paint and the backend
// never reads them.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// schema is applied on every open. Every statement is idempotent, so opening an
// existing database is a no-op and adding a column later is a plain migration.
const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id                TEXT    PRIMARY KEY,
    name              TEXT    NOT NULL,
    path              TEXT    NOT NULL,
    is_open           INTEGER NOT NULL DEFAULT 1,
    next_seq          INTEGER NOT NULL DEFAULT 1,
    active_session_id TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sessions (
    id                TEXT NOT NULL PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label             TEXT NOT NULL,
    kind              TEXT NOT NULL DEFAULT 'claude',
    path              TEXT NOT NULL DEFAULT '',
    claude_session_id TEXT NOT NULL DEFAULT '',
    label_auto        INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id);

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT NOT NULL,
    project_id TEXT NOT NULL DEFAULT '',
    value      TEXT NOT NULL,
    PRIMARY KEY (key, project_id)
);
`

// busyTimeoutMS is how long a write waits on SQLite's lock before failing.
const busyTimeoutMS = 5000

// Service owns the SQLite connection and exposes persistence to the frontend.
type Service struct {
	db *sql.DB
}

// Session is a persisted terminal session (metadata only). Kind selects what
// the PTY runs: "claude" (Claude Code binary) or "shell" (the user's shell).
// Path is the session's working directory when it lives in a git worktree;
// empty means the project's own path. ClaudeSessionID is the id Claude Code
// assigns its own session, reported by the SessionStart hook; empty until the
// hook fires (or for shell sessions), it is the key for future features that
// need to reach a session's transcript or resume it.
type Session struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Kind            string `json:"kind"`
	Path            string `json:"path"`
	ClaudeSessionID string `json:"claudeSessionId"`
}

// Project is a persisted project together with its restorable session state.
type Project struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	NextSeq         int       `json:"nextSeq"`
	ActiveSessionID string    `json:"activeSessionId"`
	Sessions        []Session `json:"sessions"`
}

// New opens (creating if absent) the SQLite database under the user's config
// directory and applies the schema.
func New() (*Service, error) {
	path, err := databasePath()
	if err != nil {
		return nil, err
	}
	return open(path)
}

// open opens the database at path, creating parent directories and applying the
// schema. foreign_keys is enabled per connection via the DSN so ON DELETE
// CASCADE fires; a single open connection serializes writes and sidesteps SQLite
// lock contention in this low-concurrency desktop app.
func open(path string) (*Service, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(%d)", path, busyTimeoutMS)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Migrations for databases created before these columns existed. SQLite has
	// no ADD COLUMN IF NOT EXISTS; a duplicate-column error means it is already
	// applied.
	migrations := []string{
		`ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'claude'`,
		`ALTER TABLE sessions ADD COLUMN path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN claude_session_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN label_auto INTEGER NOT NULL DEFAULT 1`,
	}
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil &&
			!strings.Contains(err.Error(), "duplicate column") {
			_ = db.Close()
			return nil, fmt.Errorf("migrate sessions: %w", err)
		}
	}
	return &Service{db: db}, nil
}

// databasePath resolves the on-disk location of the database file. LICH_DEV
// (set by `task dev`) selects a separate database so development migrations
// and experiments never touch the real workspace.
func databasePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	name := "lich.db"
	if os.Getenv("LICH_DEV") != "" {
		name = "lich-dev.db"
	}
	return filepath.Join(dir, "lich", name), nil
}

// Close releases the database connection.
func (s *Service) Close() error {
	return s.db.Close()
}

// LoadState returns the open projects (is_open = 1) with their sessions, ordered
// by insertion (rowid). It is the single hydration call the frontend makes on
// launch to restore the workspace.
func (s *Service) LoadState() ([]Project, error) {
	rows, err := s.db.Query(
		`SELECT id, name, path, next_seq, active_session_id
		   FROM projects WHERE is_open = 1 ORDER BY rowid`,
	)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.NextSeq, &p.ActiveSessionID); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.Sessions = []Session{}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	for i := range projects {
		sessions, err := s.sessionsOf(projects[i].ID)
		if err != nil {
			return nil, err
		}
		projects[i].Sessions = sessions
	}
	return projects, nil
}

// sessionsOf returns a project's sessions in insertion order.
func (s *Service) sessionsOf(projectID string) ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, label, kind, path, claude_session_id
		   FROM sessions WHERE project_id = ? ORDER BY rowid`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.Label, &sess.Kind, &sess.Path, &sess.ClaudeSessionID); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}
