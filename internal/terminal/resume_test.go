package terminal

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// TestResumeAvailable proves the prompt's gate answers from the transcript on
// disk, not from the id alone: a pruned conversation is what used to reach the
// PTY as the provider's own error. Each provider is asked where it files its
// own transcripts, so a live Claude id proves nothing about a Codex one.
func TestResumeAvailable(t *testing.T) {
	base := t.TempDir()
	slug := filepath.Join(base, "projects", "-home-user-proj")
	if err := os.MkdirAll(slug, 0o755); err != nil {
		t.Fatal(err)
	}
	const live = "live-uuid"
	if err := os.WriteFile(filepath.Join(slug, live+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", base)

	codexBase := t.TempDir()
	day := filepath.Join(codexBase, "sessions", "2026", "08", "09")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	const codexLive = "019fe876-0fb5-73c2-b437-b2d4bb59139e"
	rollout := filepath.Join(day, "rollout-2026-08-09T18-37-59-"+codexLive+".jsonl")
	if err := os.WriteFile(rollout, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", codexBase)

	ompBase := t.TempDir()
	// omp names the directory after the cwd the session ran in, so the id is all
	// that identifies the file — the encoding of that name is omp's business.
	ompDir := filepath.Join(ompBase, "sessions", "-home-user-proj")
	if err := os.MkdirAll(ompDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const ompLive = "019ffb38-ceab-7000-afae-20b8eae145d8"
	ompFile := filepath.Join(ompDir, "2026-08-13T13-03-51-979Z_"+ompLive+".jsonl")
	if err := os.WriteFile(ompFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", ompBase)

	// opencode keeps every conversation in one database under its data
	// directory; Crush keeps one database per checkout. The table names are
	// spelled out rather than taken from the constants, so renaming a constant
	// to match a schema lich never measured fails here instead of silently
	// passing.
	opencodeBase := t.TempDir()
	const opencodeLive = "ses_0031a382dffe1QdVbfzi6AZmbs"
	writeSessionDB(t, filepath.Join(opencodeBase, "opencode", "opencode.db"), "session", opencodeLive)
	t.Setenv("XDG_DATA_HOME", opencodeBase)

	// Antigravity answers to no environment variable of its own, so the home is
	// what has to move. Every other provider here is pointed at a directory by an
	// explicit override, so moving it changes nothing else.
	agyHome := t.TempDir()
	t.Setenv("HOME", agyHome)
	t.Setenv("USERPROFILE", agyHome)
	agyDir := filepath.Join(agyHome, ".gemini", "antigravity-cli", "conversations")
	if err := os.MkdirAll(agyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const agyLive = "7bb32ee5-e8e3-42cd-a13c-849723bc4e57"
	if err := os.WriteFile(filepath.Join(agyDir, agyLive+".db"), []byte("SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Cursor files one SQLite per chat under its config directory. The override
	// is what points it at a temporary one; its precedence is pinned separately.
	cursorBase := t.TempDir()
	const cursorLive = "bec68d79-a208-4e40-8d2f-f8f8964da216"
	cursorChat := filepath.Join(cursorBase, "chats", cursorLive)
	if err := os.MkdirAll(cursorChat, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursorChat, "store.db"), []byte("SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CURSOR_CONFIG_DIR", cursorBase)

	crushCwd := t.TempDir()
	const crushLive = "18345afc-f497-4d53-8dfd-f7c4e4d9b313"
	writeSessionDB(t, filepath.Join(crushCwd, ".crush", "crush.db"), "sessions", crushLive)
	otherCwd := t.TempDir()

	svc := &Service{}
	tests := []struct {
		name              string
		kind              string
		providerSessionID string
		cwd               string
		want              bool
	}{
		{"transcript on disk", providers.Claude, live, "", true},
		{"pruned transcript", providers.Claude, "gone-uuid", "", false},
		{"no id at all", providers.Claude, "", "", false},
		{"shell session", KindShell, live, "", false},
		{"codex rollout on disk", providers.Codex, codexLive, "", true},
		{"codex rollout deleted", providers.Codex, "019fe876-0000-0000-0000-000000000000", "", false},
		{"codex never sees a claude transcript", providers.Codex, live, "", false},
		{"omp transcript on disk", providers.OMP, ompLive, "", true},
		{"omp transcript deleted", providers.OMP, "019ffb38-0000-0000-0000-000000000000", "", false},
		{"omp never sees a codex rollout", providers.OMP, codexLive, "", false},
		{"antigravity conversation on disk", providers.Antigravity, agyLive, "", true},
		{"antigravity conversation deleted", providers.Antigravity,
			"00000000-0000-0000-0000-000000000000", "", false},
		{"antigravity never sees a claude transcript", providers.Antigravity, live, "", false},
		{"opencode row in its database", providers.OpenCode, opencodeLive, "", true},
		{"opencode conversation deleted", providers.OpenCode, "ses_gone", "", false},
		{"opencode never sees a crush row", providers.OpenCode, crushLive, "", false},
		{"crush row in the checkout's database", providers.Crush, crushLive, crushCwd, true},
		{"crush conversation deleted", providers.Crush, "00000000-0000-0000-0000-000000000000", crushCwd, false},
		// Crush's database lives in the checkout, so the same id proves nothing
		// about another one — and a session whose cwd lich cannot name has no
		// database to ask at all.
		{"crush row belongs to another checkout", providers.Crush, crushLive, otherCwd, false},
		{"crush without a working directory", providers.Crush, crushLive, "", false},
		{"cursor chat on disk", providers.Cursor, cursorLive, "", true},
		{"cursor chat deleted", providers.Cursor, "00000000-0000-0000-0000-000000000000", "", false},
		{"cursor never sees a claude transcript", providers.Cursor, live, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.ResumeAvailable(tt.kind, tt.providerSessionID, tt.cwd); got != tt.want {
				t.Errorf("ResumeAvailable(%q, %q, %q) = %v, want %v",
					tt.kind, tt.providerSessionID, tt.cwd, got, tt.want)
			}
		})
	}
}

// writeSessionDB builds the smallest database that answers the existence
// question the way a provider's own does: one table keyed by a text id, holding
// one row. The id column is all lich reads, so it is all this creates.
func writeSessionDB(t *testing.T, path, table, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE " + table + " (id TEXT PRIMARY KEY, title TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO "+table+" (id, title) VALUES (?, ?)", id, "a conversation"); err != nil {
		t.Fatal(err)
	}
}

// TestCursorConfigDirPrecedence pins the order Cursor CLI resolves its config
// directory in, and the one step that is not the xdg-basedir convention every
// other provider here follows: with no variable set it lands on ~/.cursor, never
// ~/.config/cursor. Reading it as xdg-basedir would point lich at a directory
// Cursor never writes on a machine with no XDG_CONFIG_HOME, which is most of
// them — and every resume of a Cursor session would answer "conversation gone".
func TestCursorConfigDirPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	xdg := t.TempDir()
	explicit := t.TempDir()

	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got, ok := cursorConfigDir()
	if !ok || got != filepath.Join(home, ".cursor") {
		t.Errorf("with nothing set = %q (%v), want %q", got, ok, filepath.Join(home, ".cursor"))
	}

	t.Setenv("XDG_CONFIG_HOME", xdg)
	got, ok = cursorConfigDir()
	if !ok || got != filepath.Join(xdg, "cursor") {
		t.Errorf("with XDG_CONFIG_HOME = %q (%v), want %q", got, ok, filepath.Join(xdg, "cursor"))
	}

	// The explicit override wins over XDG, and takes the directory as it stands
	// rather than hanging "cursor" off it.
	t.Setenv("CURSOR_CONFIG_DIR", explicit)
	got, ok = cursorConfigDir()
	if !ok || got != explicit {
		t.Errorf("with CURSOR_CONFIG_DIR = %q (%v), want %q", got, ok, explicit)
	}
}

// TestSessionRowExistsSurvivesAnotherToolsSchema pins the promise that makes
// reading another tool's database acceptable at all: every shape lich did not
// measure answers "no conversation" rather than erroring. A card that starts
// fresh has lost a resume; one that errors has lost the session, inside the PTY,
// with the provider's message in place of it.
func TestSessionRowExistsSurvivesAnotherToolsSchema(t *testing.T) {
	dir := t.TempDir()

	renamed := filepath.Join(dir, "renamed.db")
	writeSessionDB(t, renamed, "conversations", "live-id")
	if sessionRowExists(renamed, "session", "live-id") {
		t.Error("a renamed table answered true; the schema lich reads is not the one on disk")
	}

	notADatabase := filepath.Join(dir, "corrupt.db")
	if err := os.WriteFile(notADatabase, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if sessionRowExists(notADatabase, "session", "live-id") {
		t.Error("a file that is not a database answered true")
	}

	if sessionRowExists(filepath.Join(dir, "absent.db"), "session", "live-id") {
		t.Error("a database that is not there answered true")
	}

	empty := filepath.Join(dir, "empty-id.db")
	writeSessionDB(t, empty, "session", "live-id")
	if sessionRowExists(empty, "session", "") {
		t.Error("an empty id answered true; every row would match a LIKE, none may match this")
	}
}

// TestAntigravityConversationPath pins where an Antigravity conversation is
// looked for. The CLI reads its root through no environment variable — 1.1.19
// falls back to a hardcoded ".gemini" under the home — so the path is built from
// the home and nothing else, and a resume offered from the wrong one silently
// opens a brand new conversation instead of failing.
func TestAntigravityConversationPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	const id = "7bb32ee5-e8e3-42cd-a13c-849723bc4e57"
	if _, ok := antigravityConversationPath(id); ok {
		t.Error("answered true before the conversation existed")
	}

	dir := filepath.Join(home, ".gemini", "antigravity-cli", "conversations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, id+".db")
	if err := os.WriteFile(want, []byte("SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := antigravityConversationPath(id); !ok || got != want {
		t.Errorf("antigravityConversationPath(%q) = (%q,%v), want %q", id, got, ok, want)
	}
}

// TestOpencodeSessionDB pins where opencode's database is looked for. opencode
// applies the xdg-basedir convention on every platform rather than the
// OS-native data location, so the fallback is ~/.local/share even where the OS
// keeps application data elsewhere — resolve it the OS way and every restored
// opencode card silently starts fresh.
func TestOpencodeSessionDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	override := t.TempDir()
	t.Setenv("XDG_DATA_HOME", override)
	want := filepath.Join(override, "opencode", "opencode.db")
	if got, ok := opencodeSessionDB(); !ok || got != want {
		t.Errorf("with XDG_DATA_HOME set = (%q,%v), want %q", got, ok, want)
	}

	t.Setenv("XDG_DATA_HOME", "")
	want = filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if got, ok := opencodeSessionDB(); !ok || got != want {
		t.Errorf("with XDG_DATA_HOME unset = (%q,%v), want %q", got, ok, want)
	}
}

// TestOMPAgentDirPrecedence pins the order `omp config path` was measured to
// apply: a named profile moves the whole directory and wins over the explicit
// override, which is the opposite of the way the two read. Get it backwards and
// a profile user's resume looks at a directory omp is not writing to — a card
// that silently starts fresh every time.
func TestOMPAgentDirPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	override := t.TempDir()

	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", override)
	if got, ok := ompAgentDir(); !ok || got != override {
		t.Errorf("with the override alone = (%q,%v), want %q", got, ok, override)
	}

	t.Setenv("OMP_PROFILE", "work")
	profile := filepath.Join(home, ".omp", "profiles", "work", "agent")
	if got, ok := ompAgentDir(); !ok || got != profile {
		t.Errorf("with a profile set = (%q,%v), want %q — the profile wins", got, ok, profile)
	}

	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	fallback := filepath.Join(home, ".omp", "agent")
	if got, ok := ompAgentDir(); !ok || got != fallback {
		t.Errorf("with neither set = (%q,%v), want %q", got, ok, fallback)
	}
}
