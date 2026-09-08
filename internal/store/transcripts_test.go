package store

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// indexed wires a store to a fixed set of conversations, keyed by the provider
// session id a row carries. It stands in for terminal.TranscriptText, whose own
// reading of a provider's transcript is tested where it lives.
func indexed(t *testing.T, svc *Service, texts map[string]string) {
	t.Helper()
	svc.SetTranscriptOf(func(providerSessionID, cwd string) (string, bool) {
		return texts[providerSessionID], false
	})
}

// indexedAndCut is indexed for a conversation the cap dropped the oldest of,
// which is the answer the row has to carry through to the window.
func indexedAndCut(t *testing.T, svc *Service, texts map[string]string) {
	t.Helper()
	svc.SetTranscriptOf(func(providerSessionID, cwd string) (string, bool) {
		return texts[providerSessionID], true
	})
}

// parkWithConversation adds a session, records the provider id its conversation
// is filed under and closes it, which is what writes the index.
func parkWithConversation(t *testing.T, svc *Service, sessionID, label, providerSessionID string) {
	t.Helper()
	if err := svc.AddSession("p1", sessionID, label, "claude", "", 2, ""); err != nil {
		t.Fatalf("AddSession(%q): %v", sessionID, err)
	}
	if err := svc.SetProviderSession(sessionID, providerSessionID); err != nil {
		t.Fatalf("SetProviderSession(%q): %v", sessionID, err)
	}
	if err := svc.CloseSession("p1", sessionID, ""); err != nil {
		t.Fatalf("CloseSession(%q): %v", sessionID, err)
	}
}

// TestClosedSessionsFindsWhatWasSaid is the whole feature: a parked session
// whose name says nothing about the work is found by a phrase from inside the
// conversation, and the row carries the words that made it a hit.
func TestClosedSessionsFindsWhatWasSaid(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{
		"uuid-a": "we traced the worktree adoption guard and shipped it",
		"uuid-b": "unrelated chatter about the release notes",
	})
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")
	parkWithConversation(t, svc, "s-b", "Session 5", "uuid-b")

	closed := mustClosed(t, svc, "adoption")
	if len(closed) != 1 || closed[0].ID != "s-a" {
		t.Fatalf("ClosedSessions(adoption) = %+v, want only s-a", closed)
	}
	if !strings.Contains(closed[0].Snippet, "adoption") {
		t.Errorf("Snippet = %q, want the matched stretch of the conversation", closed[0].Snippet)
	}
}

// TestClosedSessionsMatchesNameOrConversation pins the OR the query is built
// around. A row matched on its name carries no snippet: there is no sentence
// behind that hit, and inventing one would say the conversation matched.
func TestClosedSessionsMatchesNameOrConversation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{
		"uuid-a": "nothing in here about the label",
		"uuid-b": "we traced the worktree adoption guard",
	})
	parkWithConversation(t, svc, "s-named", "worktree adoption", "uuid-a")
	parkWithConversation(t, svc, "s-said", "Session 5", "uuid-b")

	closed := mustClosed(t, svc, "worktree adoption")
	if len(closed) != 2 {
		t.Fatalf("got %d rows, want the name match and the conversation match", len(closed))
	}
	byID := map[string]ClosedSession{}
	for _, row := range closed {
		byID[row.ID] = row
	}
	if snippet := byID["s-named"].Snippet; snippet != "" {
		t.Errorf("name-matched Snippet = %q, want none", snippet)
	}
	if snippet := byID["s-said"].Snippet; !strings.Contains(snippet, "adoption") {
		t.Errorf("conversation-matched Snippet = %q, want the matched stretch", snippet)
	}
}

// TestClosedSessionsCarriesNoSnippetWithoutATerm covers the list the palette
// opens with: nothing was searched for, so no row has a sentence to show.
func TestClosedSessionsCarriesNoSnippetWithoutATerm(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{"uuid-a": "we traced the worktree adoption guard"})
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")

	closed := mustClosed(t, svc, "")
	if len(closed) != 1 {
		t.Fatalf("got %d rows, want the one parked session", len(closed))
	}
	if closed[0].Snippet != "" {
		t.Errorf("Snippet = %q, want none for a query that matched nothing in particular", closed[0].Snippet)
	}
}

// TestForgettingASessionForgetsItsConversation is the purge. A row deleted for
// good must take its conversation with it, or the index outlives the session and
// keeps answering searches with an id nothing can route to.
func TestForgettingASessionForgetsItsConversation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{"uuid-a": "we traced the worktree adoption guard"})
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")

	if err := svc.ForgetSession("s-a"); err != nil {
		t.Fatalf("ForgetSession: %v", err)
	}
	if closed := mustClosed(t, svc, "adoption"); len(closed) != 0 {
		t.Errorf("ClosedSessions(adoption) = %+v after forgetting, want nothing", closed)
	}
	if n := indexRows(t, svc); n != 0 {
		t.Errorf("%d index rows survived the delete, want 0", n)
	}
}

// TestClosingASessionAgainReplacesItsConversation covers the row that comes
// back: a resumed session is parked a second time, and appending rather than
// replacing would leave the search matching a conversation that has moved on.
func TestClosingASessionAgainReplacesItsConversation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	texts := map[string]string{"uuid-a": "the first close talked about adoption"}
	indexed(t, svc, texts)
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")

	texts["uuid-a"] = "the second close talked about ceilings"
	if err := svc.CloseSession("p1", "s-a", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if closed := mustClosed(t, svc, "adoption"); len(closed) != 0 {
		t.Errorf("ClosedSessions(adoption) = %+v, want the replaced conversation gone", closed)
	}
	if closed := mustClosed(t, svc, "ceilings"); len(closed) != 1 {
		t.Errorf("ClosedSessions(ceilings) = %+v, want the session it was said in", closed)
	}
	if n := indexRows(t, svc); n != 1 {
		t.Errorf("%d index rows for one session, want 1", n)
	}
}

// TestBackfillIndexesSessionsParkedBeforeTheIndex is the lazy catch-up: a
// workspace that parked its sessions before this version has no index for them,
// and the first search carrying a term is what starts reading them. The count
// goes back with the answer, so a list that cannot see every session yet says so
// instead of answering short in silence.
func TestBackfillIndexesSessionsParkedBeforeTheIndex(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	// Parked with nothing wired, which is what a row closed by an older lich
	// looks like: a session with a conversation on disk and no index row.
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")

	history, err := svc.ClosedSessions("adoption")
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	if history.Indexing != 1 {
		t.Fatalf("Indexing = %d before anything is wired, want 1", history.Indexing)
	}

	indexed(t, svc, map[string]string{"uuid-a": "we traced the worktree adoption guard"})
	deadline := time.Now().Add(10 * time.Second)
	for {
		history, err = svc.ClosedSessions("adoption")
		if err != nil {
			t.Fatalf("ClosedSessions: %v", err)
		}
		if history.Indexing == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("backfill still reports %d unindexed sessions", history.Indexing)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(history.Sessions) != 1 || history.Sessions[0].ID != "s-a" {
		t.Errorf("ClosedSessions(adoption) = %+v after the backfill, want s-a", history.Sessions)
	}
}

// TestBackfillMarksASessionItCannotRead is what keeps the backfill from running
// forever: a session whose conversation cannot be read is written down as read,
// so the next search does not go back to the same disk for the same nothing.
func TestBackfillMarksASessionItCannotRead(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{})
	if err := svc.AddSession("p1", "s-shell", "Home", "shell", "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.CloseSession("p1", "s-shell", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	history, err := svc.ClosedSessions("anything")
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	if history.Indexing != 0 {
		t.Errorf("Indexing = %d for a shell with no conversation, want 0", history.Indexing)
	}
}

// TestFTSQuery pins the translation from what somebody types to what FTS5 is
// asked. A term is split the way the tokenizer splits the text it searches, and
// nothing that means something to the MATCH grammar survives the quoting.
func TestFTSQuery(t *testing.T) {
	tests := []struct {
		name string
		term string
		want string
	}{
		{name: "one word is a prefix phrase", term: "adopt", want: `"adopt"*`},
		{name: "words are ANDed", term: "worktree guard", want: `"worktree"* "guard"*`},
		{
			name: "a hyphenated name splits the way the index does",
			term: "use-history-search",
			want: `"use"* "history"* "search"*`,
		},
		{name: "the MATCH grammar is not reachable", term: `a OR "b" NEAR(c)`, want: `"a"* "OR"* "b"* "NEAR"* "c"*`},
		{name: "nothing to match on", term: "  -- ", want: ""},
		{name: "an empty term", term: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ftsQuery(tt.term); got != tt.want {
				t.Errorf("ftsQuery(%q) = %q, want %q", tt.term, got, tt.want)
			}
		})
	}
}

// TestClosedSessionsSurvivesATermOfPunctuation is what the quoting above buys:
// a term FTS5 could read as an operator reaches the query as a search, not as a
// syntax error that costs the list every row.
func TestClosedSessionsSurvivesATermOfPunctuation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexed(t, svc, map[string]string{"uuid-a": "we traced the worktree adoption guard"})
	parkWithConversation(t, svc, "s-a", "Session 4", "uuid-a")

	for _, term := range []string{`"`, `*`, `(worktree`, `NOT`, `adoption OR ceilings`} {
		if _, err := svc.ClosedSessions(term); err != nil {
			t.Errorf("ClosedSessions(%q): %v", term, err)
		}
	}
}

// indexRows counts the conversations the index holds, which is the only way to
// see a row that survived a delete or a close that should have replaced it.
func indexRows(t *testing.T, svc *Service) int {
	t.Helper()
	var n int
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM session_texts`).Scan(&n); err != nil {
		t.Fatalf("count index rows: %v", err)
	}
	return n
}

// TestAroundWindowsTheConversation covers the first, cheap reading of where a
// word is: it only has to land the mention inside a window, and it must never
// hand back a half rune for the snippet to draw as a replacement character.
func TestAroundWindowsTheConversation(t *testing.T) {
	t.Run("keeps the mention and its surroundings", func(t *testing.T) {
		body := strings.Repeat("filler ", 2000) + "the ADOPTION guard " + strings.Repeat("tail ", 2000)
		got := around(body, "adoption")
		if !strings.Contains(got, "ADOPTION guard") {
			t.Errorf("around = %.80q…, want the mention and what follows it", got)
		}
		if len(got) > windowBytes {
			t.Errorf("window is %d bytes, want at most %d", len(got), windowBytes)
		}
	})

	t.Run("cuts on rune boundaries", func(t *testing.T) {
		body := strings.Repeat("é", 4000) + "adoption" + strings.Repeat("é", 4000)
		got := around(body, "adoption")
		if strings.ContainsRune(got, '�') {
			t.Error("window holds a replacement character, so an edge fell inside a rune")
		}
		if !utf8.ValidString(got) {
			t.Error("window is not valid UTF-8")
		}
	})

	t.Run("is empty when the plain text does not hold the word", func(t *testing.T) {
		// What an accent fold in the index looks like from here: the search
		// matched "cion" against "ción", and the row keeps its place with no
		// snippet under it.
		if got := around("una decisión importante", "cion"); got != "" {
			t.Errorf("around = %q, want empty", got)
		}
	})
}

// TestTheCapCuttingReachesTheRow carries the flag the whole way: the reader
// reports the cap dropped the oldest of a conversation, the close writes it down
// beside the body, and the history row the window draws says so. Nothing else
// can tell a session searched whole from one searched in part.
func TestTheCapCuttingReachesTheRow(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	indexedAndCut(t, svc, map[string]string{"uuid-a": "we traced the worktree adoption guard"})
	parkWithConversation(t, svc, "s-cut", "Session 4", "uuid-a")
	indexed(t, svc, map[string]string{"uuid-b": "a short conversation about ceilings"})
	parkWithConversation(t, svc, "s-whole", "Session 5", "uuid-b")

	byID := map[string]ClosedSession{}
	for _, row := range mustClosed(t, svc, "") {
		byID[row.ID] = row
	}
	if !byID["s-cut"].Truncated {
		t.Error("the row of a conversation the cap cut does not say so")
	}
	if byID["s-whole"].Truncated {
		t.Error("a conversation indexed whole reports itself cut")
	}
	// It rides on the row whether the search touched the conversation or not:
	// what it explains is a search that missed, which has no snippet either.
	if !mustClosedRow(t, svc, "adoption", "s-cut").Truncated {
		t.Error("the flag is lost when the row is matched on its conversation")
	}
}

// mustClosedRow is mustClosed narrowed to the one row a test is about.
func mustClosedRow(t *testing.T, svc *Service, term, sessionID string) ClosedSession {
	t.Helper()
	for _, row := range mustClosed(t, svc, term) {
		if row.ID == sessionID {
			return row
		}
	}
	t.Fatalf("ClosedSessions(%q) returned no row %q", term, sessionID)
	return ClosedSession{}
}
