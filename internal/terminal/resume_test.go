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
