package terminal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// TestResumeAvailable proves the prompt's gate answers from the transcript on
// disk, not from the id alone: a pruned conversation is what used to reach the
// PTY as Claude's own error.
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
		{"another provider", providers.Codex, live, false},
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
