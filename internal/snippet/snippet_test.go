package snippet

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAround(t *testing.T) {
	t.Run("collapses the whitespace a message is written with", func(t *testing.T) {
		got, ok := Around("a line\n\nand   another about worktrees", "worktree")
		if !ok {
			t.Fatal("no match")
		}
		if want := "a line and another about worktrees"; got != want {
			t.Errorf("snippet = %q, want %q", got, want)
		}
	})

	t.Run("windows a long message around the match", func(t *testing.T) {
		text := strings.Repeat("filler ", 200) + "the worktree port " + strings.Repeat("tail ", 200)
		got, ok := Around(text, "worktree")
		if !ok {
			t.Fatal("no match")
		}
		if !strings.Contains(got, "worktree") {
			t.Errorf("snippet %q lost the match", got)
		}
		// The window is Width runes plus the two ellipses marking that
		// both ends were cut.
		if n := utf8.RuneCountInString(got); n != Width+2 {
			t.Errorf("snippet is %d runes, want %d", n, Width+2)
		}
		if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
			t.Errorf("snippet %q does not mark what it cut", got)
		}
	})

	t.Run("marks only the end it cut", func(t *testing.T) {
		got, ok := Around("worktree "+strings.Repeat("tail ", 200), "worktree")
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
		if _, ok := Around("nothing to see", "worktree"); ok {
			t.Error("matched text that has no mention")
		}
	})
}
