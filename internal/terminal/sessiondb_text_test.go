package terminal

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// writeMessageDB plants a throwaway database and runs stmts against it, closing
// it before the path is handed back: the readers open their own read-only
// connection, and a writer still holding the file is not what they meet in
// practice.
func writeMessageDB(t *testing.T, stmts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "provider.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// opencodeMessageDB is opencode's own shape: the role on a `message` row, every
// stretch of prose a `part` row beside it keyed by the session it belongs to.
// The child session is what a `task` call files its sub-agent under — a session
// of its own, linked by `parent_id` — and its words are here to prove they are
// left out of the parent's.
func opencodeMessageDB(t *testing.T) string {
	t.Helper()
	return writeMessageDB(t,
		`CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT)`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT)`,
		`INSERT INTO session VALUES ('ses_1', NULL)`,
		`INSERT INTO session VALUES ('ses_child', 'ses_1')`,
		`INSERT INTO message VALUES ('m1','ses_1',1,'{"role":"user"}')`,
		`INSERT INTO message VALUES ('m2','ses_1',2,'{"role":"assistant"}')`,
		`INSERT INTO message VALUES ('m3','ses_child',3,'{"role":"assistant"}')`,
		`INSERT INTO part VALUES ('p1','m1','ses_1',1,'{"type":"text","text":"port the worktree hash"}')`,
		`INSERT INTO part VALUES ('p2','m2','ses_1',2,'{"type":"text","text":"An earlier worktree answer."}')`,
		`INSERT INTO part VALUES ('p3','m2','ses_1',3,'{"type":"text","text":"The worktree port is a hash."}')`,
		`INSERT INTO part VALUES ('p4','m2','ses_1',4,'{"type":"tool","tool":"bash","state":{"input":"grep worktree"}}')`,
		`INSERT INTO part VALUES ('p5','m3','ses_child',5,'{"type":"text","text":"a sub-agent worktree line"}')`,
	)
}

// crushMessageDB is Crush's own shape, which is not opencode's: one `messages`
// row per message with the role on it and every part of that message as a JSON
// array in `parts`, where a part's prose sits under `data.text` rather than
// beside its type (measured against 0.88.0). The reasoning and tool_call parts
// are here because they are what a reader matching on the row alone would
// surface instead.
func crushMessageDB(t *testing.T) string {
	t.Helper()
	return writeMessageDB(t,
		// The conversations themselves, which is what proves an id is one this
		// database has heard of (usageSourceFor).
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, cost REAL NOT NULL DEFAULT 0)`,
		`INSERT INTO sessions (id) VALUES ('crush-1'), ('crush-other')`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL,
			parts TEXT NOT NULL DEFAULT '[]', created_at INTEGER NOT NULL)`,
		`INSERT INTO messages VALUES ('m1','crush-1','user',
			'[{"type":"text","data":{"text":"port the worktree hash"}}]',1)`,
		`INSERT INTO messages VALUES ('m2','crush-1','assistant',
			'[{"type":"reasoning","data":{"thinking":"the worktree port, then"}},
			  {"type":"text","data":{"text":"An earlier worktree answer."}},
			  {"type":"finish","data":{"reason":"end_turn"}}]',2)`,
		`INSERT INTO messages VALUES ('m3','crush-1','assistant',
			'[{"type":"text","data":{"text":"The worktree port is a hash."}},
			  {"type":"tool_call","data":{"name":"bash","input":"grep worktree"}}]',3)`,
		`INSERT INTO messages VALUES ('m4','crush-other','assistant',
			'[{"type":"text","data":{"text":"another worktree session"}}]',4)`,
	)
}

// TestSaidReadsTheCrushDatabase covers the second provider whose conversation
// is a database rather than a file. The answer is the newest text part of the
// newest assistant message: the thinking that preceded it is not something the
// agent said, and another conversation's closing words are not this one's.
func TestSaidReadsTheCrushDatabase(t *testing.T) {
	src := usageSource{kind: providers.Crush, path: crushMessageDB(t), id: "crush-1"}
	if got := new(saidCursors).said("s1", src); got != "The worktree port is a hash." {
		t.Errorf("said = %q, want the newest assistant text part", got)
	}
}

// TestSearchSourceCountsAndWindowsADatabase is the search's half of the same
// mechanism: both sides of the conversation are counted, a mention that is only
// in a tool's arguments is not, and the newest match is what the palette row
// shows.
func TestSearchSourceCountsAndWindowsADatabase(t *testing.T) {
	for _, tc := range []struct {
		kind string
		path string
		id   string
	}{
		{providers.OpenCode, opencodeMessageDB(t), "ses_1"},
		{providers.Crush, crushMessageDB(t), "crush-1"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			got, ok := searchSource(usageSource{kind: tc.kind, path: tc.path, id: tc.id}, "worktree")
			if !ok {
				t.Fatal("searchSource: want a match")
			}
			want := TranscriptMatch{Snippet: "The worktree port is a hash.", Count: 3}
			if got != want {
				t.Errorf("searchSource = %+v, want %+v", got, want)
			}
		})
	}
}

// TestSearchSourceLeavesOpenCodeSubAgentsOut pins the rule the cost read makes in
// reverse: opencode files each sub-agent as a session of its own, and a hit in
// one points at a message this session never showed.
func TestSearchSourceLeavesOpenCodeSubAgentsOut(t *testing.T) {
	path := opencodeMessageDB(t)
	parent, ok := searchSource(usageSource{kind: providers.OpenCode, path: path, id: "ses_1"}, "worktree")
	if !ok || parent.Count != 3 {
		t.Fatalf("searchSource(parent) = %+v, want the parent's three mentions alone", parent)
	}
	child, ok := searchSource(usageSource{kind: providers.OpenCode, path: path, id: "ses_child"}, "worktree")
	if !ok || child.Snippet != "a sub-agent worktree line" {
		t.Fatalf("searchSource(child) = %+v, want the sub-agent's own line", child)
	}
}

// TestSessionDBTextsIsSilentForASchemaThatMoved is the direction both readers
// have to fail in: the queries are measured behaviour of another tool's schema,
// so a table or column that moves under them reads as a conversation with
// nothing in it — never as an error at the card, which is what the cost readers
// promise too.
func TestSessionDBTextsIsSilentForASchemaThatMoved(t *testing.T) {
	path := writeMessageDB(t, `CREATE TABLE messages_v2 (id TEXT PRIMARY KEY, prose TEXT)`,
		`INSERT INTO messages_v2 VALUES ('m1','a worktree line nothing here can reach')`)
	for _, kind := range []string{providers.OpenCode, providers.Crush} {
		src := usageSource{kind: kind, path: path, id: "any"}
		if got := new(saidCursors).said("s1", src); got != "" {
			t.Errorf("said(%s) = %q, want empty for a schema that moved", kind, got)
		}
		if got, ok := searchSource(src, "worktree"); ok {
			t.Errorf("searchSource(%s) = %+v, want no match for a schema that moved", kind, got)
		}
	}
	if got := sessionDBTexts(filepath.Join(t.TempDir(), "gone.db"), crushSearchQuery, "x"); got != nil {
		t.Errorf("sessionDBTexts = %v, want nil for a database that is not there", got)
	}
}
