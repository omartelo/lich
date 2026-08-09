package terminal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// userLine and assistantText build the two transcript shapes a search reads: a
// user turn, whose content is a bare string, and an assistant turn, whose
// content is always a block list.
func userLine(text string) string {
	return `{"type":"user","message":{"content":"` + text + `"}}`
}

func assistantText(text string) string {
	return `{"type":"assistant","message":{"content":[{"type":"text","text":"` + text + `"}]}}`
}

func TestSearchTranscript(t *testing.T) {
	toolCall := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash",` +
		`"input":{"command":"grep -r worktree ."}}]}}`
	toolResult := `{"type":"user","message":{"content":[{"type":"tool_result",` +
		`"content":"internal/terminal/worktreeport.go:1"}]}}`
	sidechain := `{"type":"assistant","isSidechain":true,` +
		`"message":{"content":[{"type":"text","text":"worktree from a sub-agent"}]}}`

	tests := []struct {
		name        string
		tail        string
		query       string
		wantOK      bool
		wantCount   int
		wantSnippet string
	}{
		{
			name:        "finds what the user said",
			tail:        userLine("port the worktree port hash") + "\n",
			query:       "worktree",
			wantOK:      true,
			wantCount:   1,
			wantSnippet: "port the worktree port hash",
		},
		{
			name:        "finds what the assistant said",
			tail:        assistantText("The worktree port is a hash, not a reservation.") + "\n",
			query:       "worktree",
			wantOK:      true,
			wantCount:   1,
			wantSnippet: "The worktree port is a hash, not a reservation.",
		},
		{
			name:      "matches without regard to case",
			tail:      userLine("Fix the WORKTREE port") + "\n",
			query:     "worktree",
			wantOK:    true,
			wantCount: 1,
		},
		{
			name: "keeps the newest matching message and counts every one",
			tail: userLine("first worktree question") + "\n" +
				assistantText("an answer with no mention") + "\n" +
				userLine("second worktree question") + "\n",
			query:       "worktree",
			wantOK:      true,
			wantCount:   2,
			wantSnippet: "second worktree question",
		},
		{
			// The query is in the line's JSON but not in anything that was said.
			// Surfacing it would point at a message the session never showed.
			name:   "ignores a tool call and its result",
			tail:   toolCall + "\n" + toolResult + "\n",
			query:  "worktree",
			wantOK: false,
		},
		{
			name:   "ignores a sub-agent's own conversation",
			tail:   sidechain + "\n",
			query:  "worktree",
			wantOK: false,
		},
		{
			name:   "no match is no result",
			tail:   userLine("something else entirely") + "\n",
			query:  "worktree",
			wantOK: false,
		},
		{
			// The tail starts mid-line by construction; the fragment must be
			// skipped like any malformed line, not crash the scan.
			name:        "survives a line the tail cut in half",
			tail:        `pe":"user","message":{"content":"worktree"}}` + "\n" + userLine("a worktree turn") + "\n",
			query:       "worktree",
			wantOK:      true,
			wantCount:   1,
			wantSnippet: "a worktree turn",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match, ok := searchTranscript([]byte(test.tail), test.query)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				return
			}
			if match.Count != test.wantCount {
				t.Errorf("count = %d, want %d", match.Count, test.wantCount)
			}
			if test.wantSnippet != "" && match.Snippet != test.wantSnippet {
				t.Errorf("snippet = %q, want %q", match.Snippet, test.wantSnippet)
			}
		})
	}
}

func TestSnippetAround(t *testing.T) {
	t.Run("collapses the whitespace a message is written with", func(t *testing.T) {
		got, ok := snippetAround("a line\n\nand   another about worktrees", "worktree")
		if !ok {
			t.Fatal("no match")
		}
		if want := "a line and another about worktrees"; got != want {
			t.Errorf("snippet = %q, want %q", got, want)
		}
	})

	t.Run("windows a long message around the match", func(t *testing.T) {
		text := strings.Repeat("filler ", 200) + "the worktree port " + strings.Repeat("tail ", 200)
		got, ok := snippetAround(text, "worktree")
		if !ok {
			t.Fatal("no match")
		}
		if !strings.Contains(got, "worktree") {
			t.Errorf("snippet %q lost the match", got)
		}
		// The window is snippetWidth runes plus the two ellipses marking that
		// both ends were cut.
		if n := utf8.RuneCountInString(got); n != snippetWidth+2 {
			t.Errorf("snippet is %d runes, want %d", n, snippetWidth+2)
		}
		if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
			t.Errorf("snippet %q does not mark what it cut", got)
		}
	})

	t.Run("marks only the end it cut", func(t *testing.T) {
		got, ok := snippetAround("worktree "+strings.Repeat("tail ", 200), "worktree")
		if !ok {
			t.Fatal("no match")
		}
		if strings.HasPrefix(got, "…") {
			t.Errorf("snippet %q marks a cut that did not happen", got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("snippet %q does not mark the cut end", got)
		}
	})

	t.Run("reports text that does not mention the query", func(t *testing.T) {
		if _, ok := snippetAround("nothing to see", "worktree"); ok {
			t.Error("matched text that has no mention")
		}
	})
}

func TestSearchTranscriptsRejectsAShortQuery(t *testing.T) {
	// Two characters match nearly every conversation, so the scan is refused
	// before any transcript is opened — a bare Service (nil store) proves it
	// never reaches one.
	s := &Service{}
	for _, query := range []string{"", "  ", "wo", " w "} {
		if got := s.SearchTranscripts([]string{"session"}, query); got != nil {
			t.Errorf("query %q returned %v, want nil", query, got)
		}
	}
}
