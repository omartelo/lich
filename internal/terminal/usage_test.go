// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// writeTranscript plants a Claude transcript for providerSessionID under a
// throwaway CLAUDE_CONFIG_DIR, the way claudeTranscriptPath expects to find one.
func writeTranscript(t *testing.T, providerSessionID, body string) {
	t.Helper()
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "-home-user-repo")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projects, providerSessionID+".jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
}

// writeCodexTranscript plants a rollout under CODEX_HOME and keeps the Claude
// lookup empty, proving sessionUsage falls through to the Codex reader.
func writeCodexTranscript(t *testing.T, providerSessionID, body string) {
	t.Helper()
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions", "2026", "08", "18")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessions, "rollout-now-"+providerSessionID+".jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", dir)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
}

// writeSubagentTranscript plants the transcript of one sub-agent beside the
// conversation that spawned it. It writes into the directory writeTranscript
// pointed CLAUDE_CONFIG_DIR at, so the parent is planted first.
func writeSubagentTranscript(t *testing.T, providerSessionID, agent, body string) {
	t.Helper()
	dir := filepath.Join(
		os.Getenv("CLAUDE_CONFIG_DIR"), "projects", "-home-user-repo", providerSessionID, "subagents",
	)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSessionUsageReportsTheTranscriptTail proves the whole read the card's
// context gauge rides on: session id → recorded provider session → transcript →
// the event's numbers.
func TestSessionUsageReportsTheTranscriptTail(t *testing.T) {
	writeTranscript(t, "uuid-abc", assistantLine(modelOpus, 2, 64798, 0)+"\n")
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())

	got, ok := svc.sessionUsage("s1")
	if !ok {
		t.Fatal("sessionUsage: want ok, got false")
	}
	want := usageEvent{ID: "s1", Percent: 6, Tokens: 64800, Window: 1_000_000, Model: modelOpus}
	if got != want {
		t.Errorf("sessionUsage = %+v, want %+v", got, want)
	}
}

func TestSessionUsageReportsCodexRollout(t *testing.T) {
	body := `{"type":"turn_context","payload":{"model":"gpt-5.6-sol","effort":"xhigh"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":65111},"model_context_window":258400}}}` + "\n"
	writeCodexTranscript(t, "uuid-codex", body)
	svc := New(stubBins{providerSession: "uuid-codex"}, nil, events.New())

	got, ok := svc.sessionUsage("s1")
	if !ok {
		t.Fatal("sessionUsage: want ok, got false")
	}
	want := usageEvent{
		ID: "s1", Percent: 25, Tokens: 65111, Window: 258400, Model: "gpt-5.6-sol", Effort: "xhigh",
	}
	if got != want {
		t.Errorf("sessionUsage = %+v, want %+v", got, want)
	}
}

// TestSessionUsageMissesKeepTheLastValue pins the contract every miss shares:
// no report, an unreadable store, and a transcript with nothing to read all
// yield ok=false, so the gauge holds its previous number instead of flickering
// to zero. emitUsage runs after every non-idle hook, so a miss is routine.
func TestSessionUsageMissesKeepTheLastValue(t *testing.T) {
	writeTranscript(t, "uuid-abc", assistantLine(modelOpus, 2, 64798, 0)+"\n")

	tests := []struct {
		name  string
		store stubBins
	}{
		{"no provider session reported yet", stubBins{}},
		{"the store could not be read", stubBins{providerErr: errors.New("db is gone")}},
		{"the transcript does not exist", stubBins{providerSession: "uuid-nothing"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(tc.store, nil, events.New())
			if got, ok := svc.sessionUsage("s1"); ok {
				t.Errorf("sessionUsage = %+v, want a miss", got)
			}
			// The emit wrapper must swallow the miss rather than push a zeroed
			// event — and must not panic without a connected window.
			svc.emitUsage("s1")
		})
	}
}

// TestSessionUsageCarriesTheReasoningEffort proves the effort level rides along
// with the counts: it lives on the transcript line, not inside message.usage,
// so a refactor of the parse can drop it without any count changing.
func TestSessionUsageCarriesTheReasoningEffort(t *testing.T) {
	line := `{"type":"assistant","effort":"high","message":{"model":"` + modelHaiku +
		`","usage":{"input_tokens":1000,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`
	writeTranscript(t, "uuid-effort", line+"\n")
	svc := New(stubBins{providerSession: "uuid-effort"}, nil, events.New())

	got, ok := svc.sessionUsage("s1")
	if !ok {
		t.Fatal("sessionUsage: want ok, got false")
	}
	if got.Effort != "high" || got.Window != 200_000 {
		t.Errorf("sessionUsage = %+v, want effort high and a 200k window", got)
	}
}
