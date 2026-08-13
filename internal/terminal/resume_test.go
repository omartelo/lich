package terminal

import (
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

	svc := &Service{}
	tests := []struct {
		name              string
		kind              string
		providerSessionID string
		want              bool
	}{
		{"transcript on disk", providers.Claude, live, true},
		{"pruned transcript", providers.Claude, "gone-uuid", false},
		{"no id at all", providers.Claude, "", false},
		{"shell session", KindShell, live, false},
		{"codex rollout on disk", providers.Codex, codexLive, true},
		{"codex rollout deleted", providers.Codex, "019fe876-0000-0000-0000-000000000000", false},
		{"codex never sees a claude transcript", providers.Codex, live, false},
		{"omp transcript on disk", providers.OMP, ompLive, true},
		{"omp transcript deleted", providers.OMP, "019ffb38-0000-0000-0000-000000000000", false},
		{"omp never sees a codex rollout", providers.OMP, codexLive, false},
		{"provider with no resume wired", providers.OpenCode, live, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.ResumeAvailable(tt.kind, tt.providerSessionID); got != tt.want {
				t.Errorf("ResumeAvailable(%q, %q) = %v, want %v",
					tt.kind, tt.providerSessionID, got, tt.want)
			}
		})
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
