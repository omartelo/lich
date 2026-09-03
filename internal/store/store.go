// Package store is lich's persistence layer: a single SQLite database holding
// open projects, their terminal sessions and the settings scoped globally or to
// one project — provider binaries and defaults, the permission and sandbox
// rungs, the gh account. It never stores chat or terminal content — only the
// metadata needed to restore the workspace after a restart.
//
// A UI preference stays in the frontend's localStorage by default: it needs
// synchronous access on first paint and the backend never reads it. It moves
// here when losing it costs more than reading it a frame late — the theme
// selections and the "what's new" mark did, because Chromium recreates a
// damaged profile from scratch and takes every `lich.*` key with it (the why is
// at the setting keys in frontend/src/providers/settings.tsx).
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
    active_session_id TEXT    NOT NULL DEFAULT '',
    position          INTEGER NOT NULL DEFAULT 0,
    closed_seq        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
    id                  TEXT NOT NULL PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label               TEXT NOT NULL,
    -- 'claude' is providers.Claude, which SQL cannot interpolate: renaming that
    -- constant has to move this literal and the migration below with it, or a
    -- row inserted without a kind (mutations.go's AddSession default) reads back
    -- as a provider nothing is registered under.
    kind                TEXT NOT NULL DEFAULT 'claude',
    path                TEXT NOT NULL DEFAULT '',
    provider_session_id TEXT NOT NULL DEFAULT '',
    label_auto          INTEGER NOT NULL DEFAULT 1,
    is_open             INTEGER NOT NULL DEFAULT 1,
    position            INTEGER NOT NULL DEFAULT 0,
    pinned              INTEGER NOT NULL DEFAULT 0,
    model               TEXT NOT NULL DEFAULT '',
    -- The command a terminal session opens into, empty for a plain shell. Only
    -- a kind = 'shell' row ever holds one (SetSessionEntrypoint's WHERE clause):
    -- on a provider row the entrypoint is the provider, and a value parked there
    -- would be a setting nothing reads.
    entrypoint          TEXT NOT NULL DEFAULT '',
    -- The session that asked for this one, when it was opened by delegation.
    -- Two columns rather than a foreign key: the id resolves to whatever the
    -- parent is called now, and the label is what it was called when the
    -- delegation happened — which is all that survives the parent being closed.
    origin_session_id   TEXT NOT NULL DEFAULT '',
    origin_label        TEXT NOT NULL DEFAULT '',
    -- Whether this session runs confined, when the answer belongs to the session
    -- rather than to the provider's rung: 'on', 'off', or empty to follow the
    -- setting. Three states rather than a boolean because the row has to be able
    -- to say "nobody decided this one" — a session opened before the user picked
    -- a rung, or by a caller with nowhere to ask.
    sandbox             TEXT NOT NULL DEFAULT '',
    -- When this session was parked, in unix seconds; 0 while it is open, and on
    -- a row parked before the column existed. rowid cannot stand in for it: it
    -- dates the insert, not the close, and a resume reinserts the row under a
    -- fresh id — so the history the palette lists would reorder itself every
    -- time a session came back. Seconds, not a counter like projects.closed_seq:
    -- that list only had to order, and this one has to say when.
    closed_at           INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id);

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT NOT NULL,
    project_id TEXT NOT NULL DEFAULT '',
    value      TEXT NOT NULL,
    PRIMARY KEY (key, project_id)
);

CREATE TABLE IF NOT EXISTS worktree_ports (
    path TEXT    NOT NULL PRIMARY KEY,
    port INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS session_costs (
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    transcript_id   TEXT NOT NULL,
    byte_offset     INTEGER NOT NULL DEFAULT 0,
    last_message_id TEXT NOT NULL DEFAULT '',
    cost_usd        REAL NOT NULL DEFAULT 0,
    -- When this ledger last counted a turn, in unix seconds. It is what dates a
    -- session's spend, and the only thing that can: the row is a running total
    -- with no per-turn history behind it, so a window over it selects sessions
    -- by their last counted turn rather than slicing the money by day. 0 on a
    -- row written before the column existed, which no window can place.
    updated_at      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (session_id, transcript_id)
);

CREATE TABLE IF NOT EXISTS session_hands_on (
    session_id TEXT NOT NULL PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    -- Whole seconds this session has been worked on, summed across every run of
    -- lich it has survived. Seconds rather than a finer unit because the readout
    -- is minutes: the accumulator keeps the sub-second remainder in memory and
    -- only ever hands whole seconds down (internal/terminal.handsOn).
    seconds    INTEGER NOT NULL DEFAULT 0
);
`

// busyTimeoutMS is how long a write waits on SQLite's lock before failing.
const busyTimeoutMS = 5000

// Service owns the SQLite connection and exposes persistence to the frontend.
type Service struct {
	db *sql.DB
	// sessionGone, when set, is told the id of every session whose row is
	// deleted for good. See SetSessionGone.
	sessionGone func(sessionID string)
}

// SetSessionGone registers what to run when a session's row is deleted for
// good — DeleteSession, PurgeWorktreeSessions and ForgetSession, never
// CloseSession, which parks the row for a later resume and leaves everything
// hanging off it alone.
//
// It is startup wiring, called before anything serves: lich hangs the cleanup
// of that session's dropped-file copies on it (internal/drop), which is what
// makes a copy outlive its drop and not its session.
func (s *Service) SetSessionGone(fn func(sessionID string)) {
	s.sessionGone = fn
}

// sessionIsGone reports one deleted session to whatever SetSessionGone wired,
// and nothing at all when the store runs without it — every test, and a lich
// whose wiring has not run yet.
func (s *Service) sessionIsGone(sessionIDs ...string) {
	if s.sessionGone == nil {
		return
	}
	for _, id := range sessionIDs {
		s.sessionGone(id)
	}
}

// Session is a persisted terminal session (metadata only). Kind selects what
// the PTY runs: a provider id (see internal/providers) or "shell" (the user's
// shell). Path is the session's working directory when it lives in a git
// worktree; empty means the project's own path. ProviderSessionID is the id the
// provider CLI assigns the conversation running in the PTY, reported by that
// provider's session-start hook; empty until a hook fires (or for shell
// sessions), it is the key for features that need to reach a session's
// transcript or resume it. Pinned keeps a
// session at the head of its project's list and withholds its close affordances
// until it is unpinned. OriginSessionID and OriginLabel record the session that
// asked for this one, empty for a session nobody delegated: the id names the
// parent while it is still in the workspace, the label is what that parent was
// called at the time and is all that is left once it is closed.
type Session struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	Kind              string `json:"kind"`
	Path              string `json:"path"`
	ProviderSessionID string `json:"providerSessionId"`
	// Entrypoint is the command a terminal session opens into; always empty for
	// a provider session. The window reads it to prefill its dialog and to say
	// on the card what a renamed terminal actually runs.
	Entrypoint string `json:"entrypoint"`
	// Sandbox is whether this session runs confined: "on", "off", or empty for a
	// row nothing has spawned yet. The spawn writes its own verdict here, so the
	// window can mark a confined card without re-deriving a decision that took
	// the provider's rung, the checkout and a per-session override to reach.
	Sandbox         string `json:"sandbox"`
	Pinned          bool   `json:"pinned"`
	OriginSessionID string `json:"originSessionId"`
	OriginLabel     string `json:"originLabel"`
}

// Project is a persisted project together with its restorable session state.
type Project struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	NextSeq         int       `json:"nextSeq"`
	ActiveSessionID string    `json:"activeSessionId"`
	DefaultProvider string    `json:"defaultProvider"`
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
	// no ADD COLUMN IF NOT EXISTS and no RENAME COLUMN IF EXISTS; the two errors
	// tolerated below are exactly "the column is already there" and "there is
	// nothing to rename", which is what an already-applied migration looks like.
	//
	// The rename/add pair covers all three shapes a database can be in: created
	// fresh with provider_session_id (rename finds nothing, add is a duplicate),
	// created with the old claude_session_id (rename carries the ids over, add is
	// a duplicate), or predating the column entirely (rename finds nothing, add
	// creates it).
	migrations := []string{
		// 'claude' is providers.Claude spelled out — see the schema above.
		`ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'claude'`,
		`ALTER TABLE sessions ADD COLUMN path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions RENAME COLUMN claude_session_id TO provider_session_id`,
		`ALTER TABLE sessions ADD COLUMN provider_session_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN label_auto INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE sessions ADD COLUMN is_open INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE sessions ADD COLUMN position INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN entrypoint TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN origin_session_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN origin_label TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN sandbox TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN closed_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE projects ADD COLUMN position INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE projects ADD COLUMN closed_seq INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE session_costs ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil && !migrationApplied(err) {
			_ = db.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
		}
	}
	return &Service{db: db}, nil
}

// migrationApplied reports whether an ALTER TABLE failed because the migration
// had already been applied — the column exists, or the one to rename is gone.
func migrationApplied(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "no such column")
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

// LoadState returns the open projects (is_open = 1) with their sessions, in the
// order the user dragged them into. It is the single hydration call the frontend
// makes on launch to restore the workspace.
//
// Rows that predate the position column all carry the default 0, so the rowid
// tiebreak keeps them in insertion order until a first drag assigns positions.
func (s *Service) LoadState() ([]Project, error) {
	rows, err := s.db.Query(
		`SELECT id, name, path, next_seq, active_session_id
		   FROM projects WHERE is_open = 1 ORDER BY position, rowid`,
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
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	s.loadProjectDefaults(projects)

	for i := range projects {
		sessions, err := s.sessionsOf(projects[i].ID)
		if err != nil {
			return nil, err
		}
		projects[i].Sessions = sessions
	}
	return projects, nil
}

// loadProjectDefaults hydrates every explicit project override in one query.
// Settings are supplementary workspace state: a missing table or failed read
// leaves the zero value, which means inherit, rather than blocking restoration.
func (s *Service) loadProjectDefaults(projects []Project) {
	rows, err := s.db.Query(
		`SELECT project_id, value FROM settings WHERE key = ? AND project_id <> ?`,
		providerDefaultKey, globalScope,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	defaults := make(map[string]string, len(projects))
	for rows.Next() {
		var projectID, providerID string
		if err := rows.Scan(&projectID, &providerID); err != nil {
			return
		}
		defaults[projectID] = providerID
	}
	if rows.Err() != nil {
		return
	}
	for i := range projects {
		projects[i].DefaultProvider = defaults[projects[i].ID]
	}
}

// Recent is a closed project offered for reopening — identity only, since the
// menu that lists them shows a name and a path and nothing else.
type Recent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// recentLimit caps the reopen list. The reopen menu shows five of them and the
// command palette searches all of them, so the number answers to the palette:
// far enough back to cover the projects a workspace actually cycles through,
// and still a bound — the alternative is a list that only grows and a query
// that reads every closed project ever.
const recentLimit = 25

// RecentProjects returns the closed projects (is_open = 0) offered for
// reopening, the last one closed first, up to recentLimit of them.
//
// rowid is the tiebreaker, not the order: it dates a project's first open, so
// on its own it hid a long-standing project behind newer ones the moment it was
// closed. Rows closed before closed_seq existed carry 0 and keep falling back
// to it.
func (s *Service) RecentProjects() ([]Recent, error) {
	rows, err := s.db.Query(
		`SELECT id, name, path FROM projects WHERE is_open = 0
		  ORDER BY closed_seq DESC, rowid DESC LIMIT ?`,
		recentLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent projects: %w", err)
	}
	defer rows.Close()

	recents := []Recent{}
	for rows.Next() {
		var r Recent
		if err := rows.Scan(&r.ID, &r.Name, &r.Path); err != nil {
			return nil, fmt.Errorf("scan recent project: %w", err)
		}
		recents = append(recents, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent projects: %w", err)
	}
	return recents, nil
}

// ClosedSession is one parked session offered for resuming — what identifies it
// in a list somebody is browsing rather than one they are already looking at.
// The project rides along because history spans every project at once, closed
// ones included, and the project name is what tells two sessions of the same
// name apart.
//
// No branch: it lives in git, and a worktree's directory stops agreeing with it
// the moment an agent branches inside the checkout, which is the ordinary way
// to work in one (frontend/src/lib/git/checkout-label.ts). The window reads it
// off the checkout instead, and a row whose checkout is gone has none to show —
// which is the same row that cannot be resumed anyway.
type ClosedSession struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// The project's own directory, so resuming a session of a closed project can
	// reopen that project first without a second lookup — and can ask where it
	// went when the directory has moved.
	ProjectPath string `json:"projectPath"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	// Unix seconds, 0 for a row parked before closed_at existed — which sorts
	// last and is drawn as no date rather than as 1970.
	ClosedAt int64 `json:"closedAt"`
}

// closedSessionLimit caps the history handed to the window. The palette filters
// what it was given rather than asking again per keystroke (the same bargain
// RecentProjects makes), so this number is how far back a search can reach:
// deep enough to cover the sessions a workspace churns through in months, and
// still a bound — the alternative is reading every session ever closed to draw
// a list nobody scrolls to the end of.
const closedSessionLimit = 100

// ClosedSessions returns the parked sessions (is_open = 0), the last one closed
// first, up to closedSessionLimit of them. Sessions of closed projects answer
// too: a project hidden from the tab strip still owns the work done in it, and
// resuming one of its sessions is what reopens it.
//
// rowid is the tiebreak, not the order, for RecentProjects' reason twice over:
// it dates the insert, and a resumed session is reinserted — so rows parked
// before closed_at existed all carry 0 and fall back to it together.
func (s *Service) ClosedSessions() ([]ClosedSession, error) {
	rows, err := s.db.Query(
		`SELECT s.id, s.project_id, p.name, p.path, s.label, s.kind, s.path, s.closed_at
		   FROM sessions s JOIN projects p ON p.id = s.project_id
		  WHERE s.is_open = 0
		  ORDER BY s.closed_at DESC, s.rowid DESC
		  LIMIT ?`,
		closedSessionLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("query closed sessions: %w", err)
	}
	defer rows.Close()

	closed := []ClosedSession{}
	for rows.Next() {
		var c ClosedSession
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.ProjectName, &c.ProjectPath,
			&c.Label, &c.Kind, &c.Path, &c.ClosedAt,
		); err != nil {
			return nil, fmt.Errorf("scan closed session: %w", err)
		}
		closed = append(closed, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate closed sessions: %w", err)
	}
	return closed, nil
}

// ProjectPath returns the directory of the project with this id, or "" when
// there is no such project. It is the main checkout — a session running in a
// worktree has its own directory and still belongs to this one, which is what
// makes the answer worth asking for.
func (s *Service) ProjectPath(projectID string) string {
	var path string
	if err := s.db.QueryRow(`SELECT path FROM projects WHERE id = ?`, projectID).Scan(&path); err != nil {
		return ""
	}
	return path
}

// ProjectAt names the project rooted at path — id and name — or two empty
// strings when no project is. Closed projects answer too: a row hidden from the
// tab strip still owns its directory, and relocating another project onto it
// would be the same collision.
func (s *Service) ProjectAt(path string) (string, string) {
	var id, name string
	if err := s.db.QueryRow(
		`SELECT id, name FROM projects WHERE path = ?`, path,
	).Scan(&id, &name); err != nil {
		return "", ""
	}
	return id, name
}

// sessionsOf returns a project's sessions in the order the user dragged them
// into, falling back to insertion order for rows never reordered.
//
// Pinned rows are not hoisted here, and pinning never rewrites position: this is
// the drag order, and the sidebar lifts the pinned cards over it when it draws.
// Hoisting in both places would cost a session its slot on every reload, since
// the frontend would have no order left to put an unpinned card back into.
func (s *Service) sessionsOf(projectID string) ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, label, kind, path, provider_session_id, entrypoint, sandbox, pinned,
		        origin_session_id, origin_label
		   FROM sessions WHERE project_id = ? AND is_open = 1 ORDER BY position, rowid`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var sess Session
		if err := rows.Scan(
			&sess.ID, &sess.Label, &sess.Kind, &sess.Path, &sess.ProviderSessionID,
			&sess.Entrypoint, &sess.Sandbox, &sess.Pinned, &sess.OriginSessionID, &sess.OriginLabel,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}
