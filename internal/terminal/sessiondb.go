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
func sessionRowExists(path, table, id string) bool {
	if id == "" {
		return false
	}
	db, ok := openSessionDB(path)
	if !ok {
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

// The SQL each database provider answers "what was said here" with. This is the
// second mechanism the two providers with no JSONL to walk are read through: a
// query in place of a line reader (transcriptReaderFor), returning the prose of
// one conversation and nothing else — no tool call, no thinking, no result
// written beside it.
//
// The two schemas differ in where a message keeps its parts. opencode splits one
// across `part` rows keyed by `session_id`, with the role on the `message` row
// beside them; Crush keeps every part of a message as a JSON array in the
// `messages.parts` column, with the role on the row itself, and puts a part's
// prose under `data.text` rather than beside its type. Both are measured
// behaviour of another tool's schema (opencode 1.18.23, Crush 0.88.0), so a
// column that moves reads as a conversation with nothing in it, never as an
// error at the card — the same direction the cost reads fail in.
//
// Every one of these asks for a single session id, where the cost query walks
// down into a conversation's sub-agents: opencode files each as a session of its
// own (`parent_id`) and Crush dispatches one under a session of its own too, and
// what a sub-agent said is not what this session said — its last word is not
// this one's, and a search hit in it points at a message the session never
// showed.
const (
	// The closing words: the newest text part of the newest assistant message.
	opencodeSaidQuery = `SELECT json_extract(p.data, '$.text') FROM part p
		JOIN message m ON m.id = p.message_id
		WHERE p.session_id = ? AND json_extract(m.data, '$.role') = 'assistant'
		AND json_extract(p.data, '$.type') = 'text'
		ORDER BY p.time_created DESC LIMIT 1`

	crushSaidQuery = `SELECT json_extract(part.value, '$.data.text')
		FROM messages, json_each(messages.parts) AS part
		WHERE messages.session_id = ? AND messages.role = 'assistant'
		AND json_extract(part.value, '$.type') = 'text'
		ORDER BY messages.created_at DESC, part.key DESC LIMIT 1`

	// Everything said in one conversation, oldest first, so the caller counts
	// the mentions and keeps the newest as its snippet. Both sides are returned:
	// a search is asked what was talked about, not who said it.
	opencodeSearchQuery = `SELECT json_extract(data, '$.text') FROM part
		WHERE session_id = ? AND json_extract(data, '$.type') = 'text'
		ORDER BY time_created`

	crushSearchQuery = `SELECT json_extract(part.value, '$.data.text')
		FROM messages, json_each(messages.parts) AS part
		WHERE messages.session_id = ? AND json_extract(part.value, '$.type') = 'text'
		ORDER BY messages.created_at, part.key`
)

// sessionDBQueries is the SQL one database provider answers with. Every field is
// "" for a kind that keeps no such database, and every reader below refuses the
// empty one — so a kind that reaches here by mistake reads as "nothing to show"
// rather than as free, or as silent.
type sessionDBQueries struct {
	cost   string
	said   string
	search string
}

func queriesFor(kind string) sessionDBQueries {
	switch kind {
	case providers.OpenCode:
		return sessionDBQueries{
			cost:   opencodeCostQuery,
			said:   opencodeSaidQuery,
			search: opencodeSearchQuery,
		}
	case providers.Crush:
		return sessionDBQueries{
			cost:   crushCostQuery,
			said:   crushSaidQuery,
			search: crushSearchQuery,
		}
	}
	return sessionDBQueries{}
}

// openSessionDB opens a provider's own database, or false when there is nothing
// to open. Read-only so lich can never write into a database another tool owns,
// and opened per call rather than pooled: these run on a card being restored, a
// turn ending and a key being typed, and holding a handle on a file the provider
// is migrating would be the more expensive mistake.
func openSessionDB(path string) (*sql.DB, bool) {
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", path, sessionDBBusyMS)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, false
	}
	return db, true
}

// sessionDBCost reads what one conversation cost out of a provider's own
// database. False for every failure and for a total the database cannot produce
// — a missing file, a row that is not there, a schema that moved, a NULL sum —
// which is the same "keep the last figure" the transcript readers answer with.
func sessionDBCost(path, query, id string) (float64, bool) {
	if id == "" || query == "" {
		return 0, false
	}
	db, ok := openSessionDB(path)
	if !ok {
		return 0, false
	}
	defer func() { _ = db.Close() }()

	var cost sql.NullFloat64
	if err := db.QueryRow(query, id).Scan(&cost); err != nil || !cost.Valid {
		return 0, false
	}
	return cost.Float64, true
}

// sessionDBTexts is the prose one conversation holds, in the order query asks
// for it. Nil for every failure and for a schema that no longer matches: a
// column that moved makes the query itself fail, which reads as a conversation
// with nothing in it rather than as an error at the card.
func sessionDBTexts(path, query, id string) []string {
	if id == "" || query == "" {
		return nil
	}
	db, ok := openSessionDB(path)
	if !ok {
		return nil
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(query, id)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var texts []string
	for rows.Next() {
		// NULL where the shape changed under the extract, which is a row with
		// nothing said in it rather than a read that failed.
		var text sql.NullString
		if err := rows.Scan(&text); err != nil {
			return nil
		}
		if text.String != "" {
			texts = append(texts, text.String)
		}
	}
	if rows.Err() != nil {
		return nil
	}
	return texts
}
