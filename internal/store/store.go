// Package store is lich's persistence layer: a single SQLite database holding
// open projects, their terminal sessions and the settings scoped globally or to
// one project — provider binaries and defaults, the permission and sandbox
// rungs, the gh account. The one thing it holds that is not workspace metadata
// is what a *parked* session said: a bounded copy of the conversation, indexed
// for the history search (transcripts.go), written when the session is closed
// and dropped when its row is. Nothing a live session says is ever stored; that
// is read off the provider's own transcript. The file is
// <config-dir>/lich/lich.db, or lich-dev.db under LICH_DEV so a rig never
// touches an installed lich's workspace (databasePath).
//
// A UI preference stays in the frontend's localStorage by default: it needs
// synchronous access on first paint and the backend never reads it. It moves
// here when losing it costs more than reading it a frame late — the theme
// selections and the "what's new" mark did, because Chromium recreates a
// damaged profile from scratch and takes every `lich.*` key with it (the why is
// at the setting keys in frontend/src/providers/settings.tsx). Those `lich.*`
// keys are also why the listener port is pinned rather than picked: localStorage
// is keyed by the origin the port forms, so moving it wipes the store
// (internal/singleton.DefaultPort, which LICH_LISTEN_PORT overrides; not
// LICH_PORT, the per-session hook variable).
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

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
    -- The paths under the user's home this session's sandbox skipped for being
    -- symlinks, home-relative, as a JSON array. Written by the spawn because
    -- only the spawn resolves them, and read back so the card can go on naming
    -- what a confined session will not find where it expects it — a ~/.gitconfig
    -- symlinked out of a dotfiles repository, above all. Empty for an
    -- unconfined session and for one that skipped nothing.
    sandbox_links       TEXT NOT NULL DEFAULT '',
    -- When this session was parked, in unix seconds; 0 while it is open, and on
    -- a row parked before the column existed. rowid cannot stand in for it: it
    -- dates the insert, not the close, and a resume reinserts the row under a
    -- fresh id — so the history the palette lists would reorder itself every
    -- time a session came back. Seconds, not a counter like projects.closed_seq:
    -- that list only had to order, and this one has to say when.
    closed_at           INTEGER NOT NULL DEFAULT 0,
    -- A prompt to type at this session later, and when, in unix seconds. One
    -- slot rather than a queue: scheduling again replaces what was there, which
    -- is what keeps the card able to say the whole of it in a line and this
    -- feature a reminder rather than a job runner. 0 means nothing is waiting.
    scheduled_at        INTEGER NOT NULL DEFAULT 0,
    scheduled_prompt    TEXT    NOT NULL DEFAULT '',
    -- Whether this session's last finished turn is still waiting to be read:
    -- the mark behind the card's solid ring. It is workspace state rather than a
    -- UI preference because the window is not always there to hold it. A turn
    -- that ends while the page reloads, or with lich running without a window at
    -- all, is still news when somebody comes back. Two writers own one edge each
    -- (SetSessionUnread): the terminal service, on the turn's own boundaries,
    -- and the window, when the card is read.
    unread              INTEGER NOT NULL DEFAULT 0,
    -- The MCP servers this session's provider could reach when it was spawned,
    -- as a JSON array. Written by the spawn because the answer took that
    -- provider's own config documents and this session's directory to reach, and
    -- the window needs it to divide a tool name two harnesses spell with a single
    -- underscore. Empty for a row nothing has spawned yet.
    mcp_servers         TEXT NOT NULL DEFAULT '',
    -- The branch this session's checkout was on when it was parked; empty while
    -- it is open, and on a row parked before the column existed. It exists to be
    -- searched: the history query runs in SQL over the row, and git is only
    -- asked about the rows that query already kept. It is a snapshot of the
    -- close and never a reading of now — the window still draws git's answer,
    -- because a branch moves inside a checkout while the worktree keeps the name
    -- it was created with.
    parked_branch       TEXT NOT NULL DEFAULT '',
    -- What the conversation this session was forked from had already cost when
    -- the fork was spawned, in USD, and 0 for every session that is not one. A
    -- fork's own transcript carries the history it was branched from, so the
    -- ledger counts that stretch a second time; this is what is taken back off
    -- (SaveForkCostOffset). It stays out of session_costs because it belongs to
    -- the session, not to any one transcript it has run: the fork keeps owing it
    -- after a /clear starts a second conversation under the same card.
    fork_cost_offset    REAL NOT NULL DEFAULT 0
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

-- What a parked session talked about, for the History tab's search. One row per
-- closed session, replaced by each close and written even when it is empty,
-- because an empty row is what says the session was looked at and keeps the
-- backfill from reading the same disk again on every search (transcripts.go).
--
-- The CASCADE is the whole of its deletion: a resume reinserts the session under
-- a new id and deletes the old row, and a forgotten project takes its sessions
-- with it through the same clause above — neither passes through a code path
-- that could drop the text by hand, and a body of up to eight megabytes left
-- behind on every resume is the leak that clause closes.
--
-- The body is kept here, keyed, rather than inside the index: the row a search
-- draws needs the words around its hit, and only a primary key can fetch one
-- conversation out of hundreds without walking all of them.
CREATE TABLE IF NOT EXISTS session_texts (
    session_id TEXT NOT NULL PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    -- Whether the cap on what one session's index holds dropped the oldest of
    -- this conversation (terminal.indexTextBytes). It sits before the body and
    -- not after it because that is the only way to read it without walking a
    -- blob of up to eight megabytes to reach the end of the row, and the history
    -- list reads it for every row it draws.
    truncated  INTEGER NOT NULL DEFAULT 0,
    body       TEXT NOT NULL
);

-- The index over those bodies. An index is what makes the History search
-- affordable at all: the alternative is reading every parked session's
-- transcript on every keystroke, a hundred disk reads per character on a machine
-- that can hold hundreds of conversations.
--
-- External content, so the text is stored once rather than twice, and the two
-- triggers below are what keep the index answering for the table. Every write
-- goes through transcripts.go as a delete and an insert, which is why there is
-- no update trigger: there is no update.
CREATE VIRTUAL TABLE IF NOT EXISTS session_texts_fts USING fts5(
    body, content='session_texts', content_rowid='rowid'
);
CREATE TRIGGER IF NOT EXISTS session_texts_ai AFTER INSERT ON session_texts BEGIN
    INSERT INTO session_texts_fts(rowid, body) VALUES (new.rowid, new.body);
END;
CREATE TRIGGER IF NOT EXISTS session_texts_ad AFTER DELETE ON session_texts BEGIN
    INSERT INTO session_texts_fts(session_texts_fts, rowid, body)
    VALUES ('delete', old.rowid, old.body);
END;

CREATE TABLE IF NOT EXISTS session_last_turn (
    session_id  TEXT NOT NULL PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    -- The trees a session's last finished turn ran between, and when its window
    -- shut in unix milliseconds. One row per session, replaced by each turn:
    -- the panel answers for the LAST turn and there is no history behind it.
    before_tree TEXT    NOT NULL,
    after_tree  TEXT    NOT NULL,
    ended_at    INTEGER NOT NULL DEFAULT 0
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
	// branchOf, when set, names the git branch of a checkout. See SetBranchOf.
	branchOf func(path string) string
	// transcriptOf, when set, reads a conversation as searchable prose and says
	// whether the cap dropped the oldest of it. See SetTranscriptOf.
	transcriptOf func(providerSessionID, cwd string) (text string, cut bool)
	// backfilling guards the lazy index backfill, so every search that arrives
	// while one is running starts nothing. See backfill.
	backfilling atomic.Bool
	// backfillDone and stopped are how Close waits a running backfill out
	// rather than pulling the database from under it; backfillMu orders a
	// backfill's start against that Close (backfill).
	backfillMu   sync.Mutex
	backfillDone sync.WaitGroup
	stopped      chan struct{}
	stopOnce     sync.Once
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

// SetBranchOf registers how to read a checkout's git branch (project.Branch).
// CloseSession stamps the answer on the row it parks, which is what makes the
// branch searchable in the history query.
//
// It is startup wiring, called before anything serves. Without it a park records
// no branch, which costs that row nothing but the branch as a search term.
func (s *Service) SetBranchOf(fn func(path string) string) {
	s.branchOf = fn
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
	// ScheduledAt is when the prompt below is due, in unix seconds, 0 for a
	// session with nothing waiting. It rides the session row so the card that
	// draws the countdown and the relay that delivers it read the same hydration
	// call, and neither needs a channel of its own.
	ScheduledAt     int64  `json:"scheduledAt"`
	ScheduledPrompt string `json:"scheduledPrompt"`
	// HasLastTurn is whether a last-turn record survives for this session
	// (SaveTurnRecord). It rides the hydration because the Review panel's
	// source switch is drawn before anything has asked what the turn changed:
	// without it the panel would have to wait for the session to report before
	// it could offer a record it is already holding.
	HasLastTurn bool `json:"hasLastTurn"`
	// Unread is whether this session's last finished turn is still waiting to be
	// read. It rides the hydration call because the mark has to survive a page
	// reload: the window holds it while it is up, and this row is what it comes
	// back to (see SetSessionUnread).
	Unread bool `json:"unread"`
	// MCPServers names the MCP servers this session's provider could reach when
	// it was spawned. It rides the row for the reason Sandbox does: the spawn is
	// what resolved it — from the provider's own config documents and this
	// session's directory — and a page reload has to come back to the same
	// answer without re-deriving it. Nil for a row nothing has spawned yet.
	MCPServers []string `json:"mcpServers"`
	// SandboxSkippedLinks names the paths under the user's home this session's
	// sandbox left out for being symlinks, relative to that home. It rides the
	// row for the reason MCPServers does — the spawn is what resolved it, and a
	// page reload has to come back to the same answer. Nil for an unconfined
	// session and for one whose sandbox skipped nothing.
	SandboxSkippedLinks []string `json:"sandboxSkippedLinks"`
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
		`ALTER TABLE sessions ADD COLUMN scheduled_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN scheduled_prompt TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN unread INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN mcp_servers TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN parked_branch TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN fork_cost_offset REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE sessions ADD COLUMN sandbox_links TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN position INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE projects ADD COLUMN closed_seq INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE session_costs ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE session_texts ADD COLUMN truncated INTEGER NOT NULL DEFAULT 0`,
		// The terminal's own theme selection, from before one theme coloured both
		// surfaces. Nothing reads it, and a row left standing would hand whoever
		// gives the terminal a theme again a choice its user made under other
		// rules. Unlike the ALTERs above this runs on every launch, so a terminal
		// theme that ever comes back needs a key of its own: this one is cleared
		// under it.
		`DELETE FROM settings WHERE key = 'appearance.terminalTheme'`,
	}
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil && !migrationApplied(err) {
			_ = db.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
		}
	}
	return &Service{db: db, stopped: make(chan struct{})}, nil
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

// Close releases the database connection, after waiting out a backfill still
// indexing parked sessions (transcripts.go). Waiting rather than racing: the
// goroutine writes through this same connection, and closing it under one is a
// failed write logged as if the disk had gone.
func (s *Service) Close() error {
	s.backfillMu.Lock()
	s.stopOnce.Do(func() { close(s.stopped) })
	s.backfillMu.Unlock()
	s.backfillDone.Wait()
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

// recentLimit caps how many closed projects one call answers with, not how far
// back it looks: the search runs in the query, so a project closed long before
// the newest twenty-five is still found by name. The alternative is a list that
// only grows and a menu that reads every closed project ever.
const recentLimit = 25

// closedProjectMatch is the WHERE clause and its arguments for the closed
// projects (is_open = 0) matching term: every whitespace-separated word has to
// appear somewhere in the name or the path, which is the same reading the
// palette's own filter gives a query. An empty term matches all of them. LIKE
// is case-insensitive for ASCII in SQLite, which is what makes this a search
// and not a prefix test.
func closedProjectMatch(term string) (string, []any) {
	where := "WHERE is_open = 0"
	args := []any{}
	for _, word := range strings.Fields(term) {
		where += " AND (name || ' ' || path) LIKE ? ESCAPE '" + likeEscape + "'"
		args = append(args, "%"+escapeLike(word)+"%")
	}
	return where, args
}

// RecentProjects returns the closed projects matching term, the last one closed
// first, up to recentLimit of them. An empty term is the plain reopen list: the
// most recently closed, whatever they are named.
//
// The term is matched here rather than in the window for the reason
// ClosedSessions matches its own: the window only ever sees one page of rows, so
// a project closed further back than that page would be unreachable by name.
//
// rowid is the tiebreaker, not the order: it dates a project's first open, so
// on its own it hid a long-standing project behind newer ones the moment it was
// closed. Rows closed before closed_seq existed carry 0 and keep falling back
// to it.
func (s *Service) RecentProjects(term string) ([]Recent, error) {
	where, args := closedProjectMatch(term)
	args = append(args, recentLimit)
	rows, err := s.db.Query(
		`SELECT id, name, path FROM projects `+where+`
		  ORDER BY closed_seq DESC, rowid DESC LIMIT ?`,
		args...,
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

// ClosedProjectCount counts the closed projects matching term, which is how a
// list capped at recentLimit says what it is leaving out: the reopen menu points
// at the palette when there are more, and the palette's own group header reports
// the matches it could not fit. A COUNT over one indexed flag is cheap enough to
// ask for beside the list itself.
func (s *Service) ClosedProjectCount(term string) (int, error) {
	where, args := closedProjectMatch(term)
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects `+where, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count closed projects: %w", err)
	}
	return count, nil
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
		        origin_session_id, origin_label, scheduled_at, scheduled_prompt, unread,
		        mcp_servers, sandbox_links,
		        EXISTS (SELECT 1 FROM session_last_turn WHERE session_id = sessions.id)
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
		var servers, links string
		if err := rows.Scan(
			&sess.ID, &sess.Label, &sess.Kind, &sess.Path, &sess.ProviderSessionID,
			&sess.Entrypoint, &sess.Sandbox, &sess.Pinned, &sess.OriginSessionID, &sess.OriginLabel,
			&sess.ScheduledAt, &sess.ScheduledPrompt, &sess.Unread, &servers, &links,
			&sess.HasLastTurn,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sess.MCPServers = decodeStrings(servers)
		sess.SandboxSkippedLinks = decodeStrings(links)
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}
