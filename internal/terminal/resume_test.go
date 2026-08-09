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
