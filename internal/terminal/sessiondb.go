package terminal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
	_ "modernc.org/sqlite"
)

// Where opencode and Crush keep their conversations. Neither files one
// transcript per session the way Claude Code, Codex and oh-my-pi do, so there is
// nothing to glob for: the proof a conversation still exists is a row in the
// provider's own SQLite database. Only the id column of one table is read, and
// only to answer whether the row is there.
//
// This fails in the same direction the transcript globs do, which is what makes
// it safe to depend on another tool's schema: a database that moved, a table
// that was renamed, a file that cannot be opened — every one of them answers
// false, so a restored card starts fresh instead of dying inside the PTY with
// the provider's error in place of a session.
const (
	// opencodeSessionTable is singular and Crush's is plural. Both are keyed by
	// a text `id`, which is the id the session-start hook reported.
	opencodeSessionTable = "session"
	crushSessionTable    = "sessions"

	// sessionDBBusyMS caps the wait for a database the provider is writing to
	// right now. A read that gives up reads as "no conversation", which costs a
	// resume that was there, so it is worth waiting out a lock; it is short
	// enough that the spawn gate never feels stuck.
	sessionDBBusyMS = 500
)

// opencodeSessionDB is the single database opencode keeps every conversation in,
// under its data directory. opencode reads that directory through the
// xdg-basedir convention on every platform rather than the OS-native data
// location, the same way it resolves the plugin directory
// (internal/agentplugin/opencode.go) — so os.UserCacheDir and friends would
// point somewhere opencode never writes.
func opencodeSessionDB() (string, bool) {
	base, ok := harnessDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
	if !ok {
		return "", false
	}
	return filepath.Join(base, "opencode", "opencode.db"), true
}

// crushSessionDB is the database Crush keeps in the checkout it was started in.
// cwd is the session's own working directory, which is the directory lich
// spawned Crush in. False without one: a directory lich cannot name has no
// database to ask, and answering from the process's own cwd would be asking
// about somebody else's checkout.
func crushSessionDB(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	return filepath.Join(cwd, ".crush", "crush.db"), true
}

// sessionRowExists reports whether table in the SQLite database at path holds a
// row with this id. False for every failure — a missing file, a database held
// exclusively, a table that is not there any more.
//
// The connection is read-only so lich can never write into a database another
// tool owns, and it is opened per call rather than pooled: this runs once when a
// restored card is first opened, and holding a handle on a file the provider is
// migrating would be the more expensive mistake.
func sessionRowExists(path, table, id string) bool {
	if id == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", path, sessionDBBusyMS)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return false
	}
	defer func() { _ = db.Close() }()

	// table is a constant from this file and never anything that arrived from
	// outside lich; the id is bound.
	var found int
	query := "SELECT 1 FROM " + table + " WHERE id = ? LIMIT 1"
	return db.QueryRow(query, id).Scan(&found) == nil
}

// The SQL each database provider answers "what has this conversation cost" with,
// in USD, from the figure it billed at itself. Neither is re-priced from a table
// here: both bill models no table in lich knows, and the number on screen is the
// one their own UI shows.
const (
	// opencode files each sub-agent as a session of its own and leaves the
	// parent's cost its own turns alone — measured against 1.18.23, where a task
	// call left the parent at $0.021 and the child at $0.0105 — so the total
	// walks down the parent chain. Recursive rather than one level deep: a
	// sub-agent can call the task tool again.
	//
	// SUM over no rows is NULL, which is how a conversation this database has
	// never heard of is told apart from one that has genuinely cost nothing.
	opencodeCostQuery = `WITH RECURSIVE tree(id) AS (
		SELECT id FROM session WHERE id = ?
		UNION SELECT s.id FROM session s JOIN tree t ON s.parent_id = t.id
	) SELECT SUM(s.cost) FROM session s JOIN tree t ON s.id = t.id`

	// Crush rolls a sub-agent's spend into the session that dispatched it
	// (`updateParentSessionCost`), so one row is the whole number — measured
	// against 0.88.0, where a parent billed $0.0525 against its child's $0.021.
	crushCostQuery = `SELECT cost FROM sessions WHERE id = ?`
)

// costQueryFor is the query above for a provider id, and "" for a provider that
// keeps no such database. sessionDBCost refuses the empty one, so a kind that
// reaches here by mistake reads as "nothing to show" rather than as free.
func costQueryFor(kind string) string {
	switch kind {
	case providers.OpenCode:
		return opencodeCostQuery
	case providers.Crush:
		return crushCostQuery
	}
	return ""
}

// sessionDBCost reads what one conversation cost out of a provider's own
// database. False for every failure and for a total the database cannot produce
// — a missing file, a row that is not there, a schema that moved, a NULL sum —
// which is the same "keep the last figure" the transcript readers answer with.
//
// Read-only and opened per call, for the reasons sessionRowExists gives: lich
// must never write into a database another tool owns, and holding a handle on
// one being migrated would be the more expensive mistake.
func sessionDBCost(path, query, id string) (float64, bool) {
	if id == "" || query == "" {
		return 0, false
	}
	if _, err := os.Stat(path); err != nil {
		return 0, false
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", path, sessionDBBusyMS)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return 0, false
	}
	defer func() { _ = db.Close() }()

	var cost sql.NullFloat64
	if err := db.QueryRow(query, id).Scan(&cost); err != nil || !cost.Valid {
		return 0, false
	}
	return cost.Float64, true
}
