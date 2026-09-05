package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// searchStore answers the one Store method the search reaches. The embedded
// interface is nil on purpose: a search that starts calling anything else panics
// here instead of quietly reading a zero value.
type searchStore struct {
	Store
	providerSessions map[string]string
	errs             map[string]error
}

func (s searchStore) ProviderSession(sessionID string) (string, error) {
	return s.providerSessions[sessionID], s.errs[sessionID]
}

// plantTranscripts writes one Claude transcript per provider session id under a
// throwaway CLAUDE_CONFIG_DIR, the way claudeTranscriptPath expects to find them.
func plantTranscripts(t *testing.T, bodies map[string]string) {
	t.Helper()
	dir := t.TempDir()
	projects := filepath.Join(dir, "projects", "-home-user-repo")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	for providerSessionID, body := range bodies {
		path := filepath.Join(projects, providerSessionID+".jsonl")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
}

// TestSearchTranscriptsWalksEverySession proves the whole read a palette search
// rides on: each session id → its recorded provider session → that transcript →
// the match, carrying the id the caller can route to. The sessions with nothing
// to search (no provider session, an unreadable store row, a transcript that is
// gone, a conversation with no mention) are in the same list: every miss is
// silent and simply contributes no result.
func TestSearchTranscriptsWalksEverySession(t *testing.T) {
	plantTranscripts(t, map[string]string{
		"uuid-a": userLine("port the worktree port hash") + "\n",
		"uuid-b": assistantText("The worktree port is a hash, not a reservation.") + "\n",
		"uuid-c": userLine("something else entirely") + "\n",
	})
	store := searchStore{
		providerSessions: map[string]string{
			"s-a":     "uuid-a",
			"s-b":     "uuid-b",
			"s-c":     "uuid-c",
			"s-gone":  "uuid-never-written",
			"s-shell": "",
		},
		errs: map[string]error{"s-broken": errors.New("db is gone")},
	}
	svc := New(store, nil, events.New())

	// Mixed case and padding: the query is normalised once, before any
	// transcript is opened.
	got := svc.SearchTranscripts([]string{"s-a", "s-broken", "s-shell", "s-c", "s-gone", "s-b"}, "  WorkTree ")
	want := []TranscriptMatch{
		{ID: "s-a", Snippet: "port the worktree port hash", Count: 1},
		{ID: "s-b", Snippet: "The worktree port is a hash, not a reservation.", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SearchTranscripts = %+v, want %+v", got, want)
	}
}

// TestSearchTranscriptsFindsANonClaudeSession proves the chain a search rides for
// a provider that is not Claude Code: the id resolves to no Claude
// transcript, usageSourceFor finds the Codex rollout filed under it instead, and
// the rollout's own block names are read. Planting both stores at once is the
// point — the search asks the disk which provider ran a conversation, exactly as
// the cost and context readouts do.
func TestSearchTranscriptsFindsANonClaudeSession(t *testing.T) {
	plantTranscripts(t, map[string]string{"uuid-claude": userLine("a claude worktree turn") + "\n"})
	plantCodexRollout(t, "uuid-codex",
		`{"type":"response_item","payload":{"type":"message","role":"assistant",`+
			`"content":[{"type":"output_text","text":"The worktree port is a hash."}]}}`+"\n")
	store := searchStore{providerSessions: map[string]string{
		"s-claude": "uuid-claude",
		"s-codex":  "uuid-codex",
	}}
	svc := New(store, nil, events.New())

	got := svc.SearchTranscripts([]string{"s-claude", "s-codex"}, "worktree")
	want := []TranscriptMatch{
		{ID: "s-claude", Snippet: "a claude worktree turn", Count: 1},
		{ID: "s-codex", Snippet: "The worktree port is a hash.", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SearchTranscripts = %+v, want %+v", got, want)
	}
}

// plantCodexRollout writes one rollout under a throwaway CODEX_HOME, in the
// dated directory Codex files them in.
func plantCodexRollout(t *testing.T, providerSessionID, body string) {
	t.Helper()
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions", "2026", "09", "05")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessions, "rollout-2026-09-05T10-00-00-"+providerSessionID+".jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", dir)
}

// TestSnippetAroundSurvivesGrowingCaseFolding pins the offset contract of the
// snippet: lowercasing "Ⱥ" yields a longer rune, so a match offset taken from a
// lowered copy runs past the end of the text it is meant to index.
func TestSnippetAroundSurvivesGrowingCaseFolding(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("Ⱥ", 200) + " worktree",
		strings.Repeat("İ", 200) + " worktree",
	} {
		got, ok := snippetAround(text, "worktree")
		if !ok {
			t.Fatalf("snippetAround(%.4q…) found no match", text)
		}
		if !strings.Contains(got, "worktree") {
			t.Errorf("snippet %q lost the match", got)
		}
	}
}
