package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// conversationHome points every provider's store at one throwaway home, with
// no variable left to move a root somewhere else.
func conversationHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, v := range []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME",
		"CURSOR_CONFIG_DIR", "OMP_PROFILE", "PI_CODING_AGENT_DIR",
	} {
		t.Setenv(v, "")
	}
	return home
}

func plant(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func plantDB(t *testing.T, path string, stmts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(writeMessageDB(t, stmts...), path); err != nil {
		t.Fatal(err)
	}
}

// byID drops the timestamps, which come off file mtimes, so a table can pin the
// rest exactly.
func byID(cs []store.Conversation) map[string]store.Conversation {
	out := map[string]store.Conversation{}
	for _, c := range cs {
		c.UpdatedAt = 0
		out[c.ID] = c
	}
	return out
}

func assertConversations(t *testing.T, got []store.Conversation, want ...store.Conversation) {
	t.Helper()
	gotByID := byID(got)
	if len(gotByID) != len(want) {
		t.Fatalf("conversations = %+v, want %+v", got, want)
	}
	for _, w := range want {
		if gotByID[w.ID] != w {
			t.Errorf("conversation %s = %+v, want %+v", w.ID, gotByID[w.ID], w)
		}
	}
}

func TestClaudeConversations(t *testing.T) {
	home := conversationHome(t)
	dir := filepath.Join(home, ".claude", "projects", "-work-alpha")
	plant(t, filepath.Join(dir, "titled.jsonl"),
		`{"type":"custom-title","customTitle":"fix login","sessionId":"titled"}
{"type":"user","cwd":"/work/alpha","entrypoint":"cli","message":{"content":"make login work"}}
`)
	plant(t, filepath.Join(dir, "untitled.jsonl"),
		`{"type":"user","cwd":"/work/alpha","isMeta":true,"message":{"content":"<local-command-caveat>x</local-command-caveat>"}}
{"type":"user","cwd":"/work/alpha","message":{"content":"<command-name>/clear</command-name>"}}
{"type":"user","cwd":"/work/alpha","message":{"content":[{"type":"tool_result"}]}}
{"type":"user","cwd":"/work/alpha","message":{"content":"why is the build red\nsecond line"}}
`)
	plant(t, filepath.Join(dir, "ai.jsonl"),
		`{"type":"user","cwd":"/work/alpha","message":{"content":"hello"}}
{"type":"ai-title","aiTitle":"Greeting"}
`)
	plant(t, filepath.Join(dir, "lich-born.jsonl"),
		`{"type":"custom-title","customTitle":"alpha-3f9a"}
{"type":"user","cwd":"/work/alpha","message":{"content":"from lich"}}
`)
	plant(t, filepath.Join(dir, "headless.jsonl"),
		`{"type":"user","cwd":"/work/alpha","entrypoint":"sdk-cli","message":{"content":"claude -p"}}
`)
	plant(t, filepath.Join(dir, "no-cwd.jsonl"), `{"type":"summary"}`+"\n")
	plant(t, filepath.Join(dir, "titled", "subagents", "agent-1.jsonl"),
		`{"type":"user","cwd":"/work/alpha","message":{"content":"sub"}}`+"\n")

	assertConversations(t, claudeConversations(),
		store.Conversation{Kind: "claude", ID: "titled", Title: "fix login", Cwd: "/work/alpha"},
		store.Conversation{Kind: "claude", ID: "untitled", Title: "why is the build red", Cwd: "/work/alpha"},
		store.Conversation{Kind: "claude", ID: "ai", Title: "Greeting", Cwd: "/work/alpha"},
	)
}

func TestCodexConversations(t *testing.T) {
	home := conversationHome(t)
	day := filepath.Join(home, ".codex", "sessions", "2026", "09", "14")
	plant(t, filepath.Join(day, "rollout-2026-09-14T21-26-37-c1.jsonl"),
		`{"type":"session_meta","payload":{"id":"c1","cwd":"/work/alpha","source":"cli","thread_source":"user"}}
{"type":"event_msg","payload":{"type":"user_message","message":"port the theme loader"}}
`)
	plant(t, filepath.Join(day, "rollout-2026-09-14T21-30-00-c2.jsonl"),
		`{"type":"session_meta","payload":{"id":"c2","cwd":"/work/alpha","source":"cli"}}
{"type":"event_msg","payload":{"type":"user_message","message":"unnamed prompt"}}
`)
	plant(t, filepath.Join(day, "rollout-2026-09-14T21-31-00-exec.jsonl"),
		`{"type":"session_meta","payload":{"id":"exec","cwd":"/work/alpha","source":"exec"}}`+"\n")
	plant(t, filepath.Join(day, "rollout-2026-09-14T21-32-00-sub.jsonl"),
		`{"type":"session_meta","payload":{"id":"sub","cwd":"/work/alpha","source":{"subagent":"x"},"thread_source":"subagent"}}`+"\n")
	plant(t, filepath.Join(day, "rollout-2026-09-14T21-33-00-bad.jsonl"), "not json\n")
	plant(t, filepath.Join(home, ".codex", "session_index.jsonl"),
		`{"id":"c2","thread_name":"Named by the user"}`+"\n")

	assertConversations(t, codexConversations(),
		store.Conversation{Kind: "codex", ID: "c1", Title: "port the theme loader", Cwd: "/work/alpha"},
		store.Conversation{Kind: "codex", ID: "c2", Title: "Named by the user", Cwd: "/work/alpha"},
	)
}

func TestOMPConversations(t *testing.T) {
	home := conversationHome(t)
	dir := filepath.Join(home, ".omp", "agent", "sessions", "-work-alpha")
	plant(t, filepath.Join(dir, "2026-09-15T00-24-17-727Z_o1.jsonl"),
		`{"type":"title","title":"Find relay deadline test"}
{"type":"session","id":"o1","cwd":"/work/alpha"}
`)
	plant(t, filepath.Join(dir, "2026-09-15T00-25-00-000Z_o2.jsonl"),
		`{"type":"session","id":"o2","cwd":"/work/alpha"}`+"\n")
	plant(t, filepath.Join(dir, "2026-09-15T00-26-00-000Z_o3.jsonl"), `{"type":"title","title":"no header"}`+"\n")
	plant(t, filepath.Join(dir, "o1", "2026-09-15T00-27-00-000Z_sub.jsonl"),
		`{"type":"session","id":"sub","cwd":"/work/alpha"}`+"\n")

	assertConversations(t, ompConversations(),
		store.Conversation{Kind: "omp", ID: "o1", Title: "Find relay deadline test", Cwd: "/work/alpha"},
		store.Conversation{Kind: "omp", ID: "o2", Cwd: "/work/alpha"},
	)
}

func TestKiroConversations(t *testing.T) {
	home := conversationHome(t)
	dir := filepath.Join(home, ".kiro", "sessions", "cli")
	plant(t, filepath.Join(dir, "k1.json"),
		`{"session_id":"k1","cwd":"/work/alpha","title":"fix datasource","updated_at":"2026-09-04T14:42:12.911320614Z","session_created_reason":"subagent"}`)
	plant(t, filepath.Join(dir, "k2.json"),
		`{"session_id":"k2","cwd":"/work/alpha","title":null,"updated_at":"garbled"}`)
	plant(t, filepath.Join(dir, "k3.json"), `{"session_id":"k3"}`)

	got := kiroConversations()
	assertConversations(t, got,
		store.Conversation{Kind: "kiro", ID: "k1", Title: "fix datasource", Cwd: "/work/alpha"},
		store.Conversation{Kind: "kiro", ID: "k2", Cwd: "/work/alpha"},
	)
	for _, c := range got {
		if c.ID == "k1" && c.UpdatedAt != 1788532932 {
			t.Errorf("k1 UpdatedAt = %d, want the recorded updated_at", c.UpdatedAt)
		}
		if c.ID == "k2" && c.UpdatedAt == 0 {
			t.Error("k2 UpdatedAt = 0, want the file's mtime when updated_at does not parse")
		}
	}
}

func TestCursorConversations(t *testing.T) {
	home := conversationHome(t)
	chats := filepath.Join(home, ".cursor", "chats", "0123abcd")
	plant(t, filepath.Join(chats, "u1", "meta.json"),
		`{"cwd":"/work/alpha","updatedAtMs":1787675503922,"hasConversation":true}`)
	plant(t, filepath.Join(chats, "u1", "prompt_history.json"), `["/model","which providers do you route?"]`)
	plant(t, filepath.Join(chats, "u2", "meta.json"),
		`{"cwd":"/work/alpha","updatedAtMs":1,"hasConversation":true}`)
	plant(t, filepath.Join(chats, "empty", "meta.json"),
		`{"cwd":"/work/alpha","updatedAtMs":1,"hasConversation":false}`)

	got := cursorConversations()
	assertConversations(t, got,
		store.Conversation{Kind: "cursor", ID: "u1", Title: "which providers do you route?", Cwd: "/work/alpha"},
		store.Conversation{Kind: "cursor", ID: "u2", Cwd: "/work/alpha"},
	)
	if i := slices.IndexFunc(got, func(c store.Conversation) bool { return c.ID == "u1" }); got[i].UpdatedAt != 1787675503 {
		t.Errorf("u1 UpdatedAt = %d, want updatedAtMs in seconds", got[i].UpdatedAt)
	}
}

func TestAntigravityConversations(t *testing.T) {
	home := conversationHome(t)
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	plantDB(t, filepath.Join(root, "conversation_summaries.db"),
		`CREATE TABLE conversation_summaries (conversation_id TEXT, title TEXT, preview TEXT,
		   workspace_uris TEXT, parent_conversation_id TEXT)`,
		`INSERT INTO conversation_summaries VALUES
		   ('a1', 'Live chat', '', '["file:///work/alpha","file:///work/beta"]', ''),
		   ('a2', '', 'what does this repo do', '["file:///work/alpha"]', ''),
		   ('child', 'Sub', '', '["file:///work/alpha"]', 'a1'),
		   ('gone', 'No database', '', '["file:///work/alpha"]', ''),
		   ('nowhere', 'No workspace', '', '[]', '')`)
	for _, id := range []string{"a1", "a2", "child", "nowhere"} {
		plant(t, filepath.Join(root, "conversations", id+".db"), "")
	}

	assertConversations(t, antigravityConversations(),
		store.Conversation{Kind: "antigravity", ID: "a1", Title: "Live chat", Cwd: filepath.FromSlash("/work/alpha")},
		store.Conversation{Kind: "antigravity", ID: "a2", Title: "what does this repo do", Cwd: filepath.FromSlash("/work/alpha")},
	)
}

func TestOpencodeConversations(t *testing.T) {
	home := conversationHome(t)
	plantDB(t, filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
		`CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT, directory TEXT, title TEXT,
		   time_updated INTEGER, time_archived INTEGER)`,
		`INSERT INTO session VALUES
		   ('ses_1', NULL, '/work/alpha', 'New session', 1786468293571, NULL),
		   ('ses_child', 'ses_1', '/work/alpha', 'task', 1786468293571, NULL),
		   ('ses_archived', NULL, '/work/alpha', 'old', 1, 2)`)

	got := opencodeConversations()
	assertConversations(t, got,
		store.Conversation{Kind: "opencode", ID: "ses_1", Title: "New session", Cwd: "/work/alpha"})
	if got[0].UpdatedAt != 1786468293 {
		t.Errorf("UpdatedAt = %d, want time_updated in seconds", got[0].UpdatedAt)
	}
}

func TestCrushConversations(t *testing.T) {
	home := conversationHome(t)
	checkout := filepath.Join(home, "work", "alpha")
	plant(t, filepath.Join(home, ".local", "share", "crush", "projects.json"),
		`{"projects":[{"path":"`+filepath.ToSlash(checkout)+`","data_dir":"elsewhere"},{"path":"/no/database"}]}`)
	plantDB(t, filepath.Join(checkout, ".crush", "crush.db"),
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, parent_session_id TEXT, title TEXT, updated_at INTEGER)`,
		`INSERT INTO sessions VALUES ('r1', NULL, 'why is the sky blue', 1786393727),
		   ('r2', '', 'empty parent', 1), ('task', 'r1', 'sub', 1)`)

	assertConversations(t, crushConversations(),
		store.Conversation{Kind: "crush", ID: "r1", Title: "why is the sky blue", Cwd: filepath.ToSlash(checkout)},
		store.Conversation{Kind: "crush", ID: "r2", Title: "empty parent", Cwd: filepath.ToSlash(checkout)},
	)
}

// TestConversationsReadsEveryProvider pins the Registry checklist: a provider
// lich runs and cannot list is a gap somebody has to have chosen.
func TestConversationsReadsEveryProvider(t *testing.T) {
	conversationHome(t)
	for _, kind := range []string{"claude", "codex", "antigravity", "opencode", "omp", "crush", "cursor", "kiro"} {
		if _, ok := conversationReaders[kind]; !ok {
			t.Errorf("no conversation reader for %s", kind)
		}
	}
	if got := Conversations(); len(got) != 0 {
		t.Errorf("Conversations on an empty home = %+v, want none", got)
	}
}

func TestFirstWorkspaceKeepsAWindowsDrive(t *testing.T) {
	if got := firstWorkspace(`["file:///C:/work/alpha"]`); got != filepath.FromSlash("C:/work/alpha") {
		t.Errorf("firstWorkspace = %q, want the drive kept", got)
	}
	if got := firstWorkspace(`["https://example.com/x"]`); got != "" {
		t.Errorf("firstWorkspace(https) = %q, want nothing", got)
	}
}

func TestPromptTitleCutsALongPromptToOneLine(t *testing.T) {
	long := strings.Repeat("ã", conversationTitleRunes+10)
	if got := promptTitle(long); got != strings.Repeat("ã", conversationTitleRunes)+"…" {
		t.Errorf("promptTitle(long) = %q", got)
	}
	if got := promptTitle("  first\nsecond"); got != "first" {
		t.Errorf("promptTitle = %q, want the first line", got)
	}
}

// TestHeadLinesDropsTheLineTheBoundCuts pins the bound: a record cut in half
// by it would otherwise reach the parser as a different, shorter record.
func TestHeadLinesDropsTheLineTheBoundCuts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.jsonl")
	first := `{"type":"session"}`
	plant(t, path, first+"\n"+strings.Repeat("x", conversationHeadBytes)+"\n")
	lines := headLines(path)
	if len(lines) != 1 || string(lines[0]) != first {
		t.Errorf("headLines = %d lines, want only the complete first one", len(lines))
	}
}
