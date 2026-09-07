package store

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// atClock stamps every close during fn with the given wall clock, so a test can
// order two closes without waiting out a real second.
func atClock(t *testing.T, at time.Time, fn func()) {
	t.Helper()
	previous := now
	now = func() time.Time { return at }
	defer func() { now = previous }()
	fn()
}

// TestClosedSessionsOrdersByCloseNotByInsert is the whole reason closed_at
// exists: rowid dates the insert, so a session opened first and closed last
// would sort to the bottom of a list that claims to be "most recently closed".
func TestClosedSessionsOrdersByCloseNotByInsert(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "first", "opened first", "claude", "/wt/a", 3, "")
	_ = svc.AddSession("p1", "second", "opened second", "claude", "/wt/b", 4, "")

	// The one inserted first is closed last, so insert order and close order
	// disagree on every row.
	atClock(t, time.Unix(1_700_000_100, 0), func() { _ = svc.CloseSession("p1", "second", "keep") })
	atClock(t, time.Unix(1_700_000_200, 0), func() { _ = svc.CloseSession("p1", "first", "keep") })

	closed, err := svc.ClosedSessions("")
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	if len(closed) != 2 {
		t.Fatalf("got %d closed sessions, want 2", len(closed))
	}
	if closed[0].ID != "first" || closed[1].ID != "second" {
		t.Errorf("order = %q, %q; want the last one closed first", closed[0].ID, closed[1].ID)
	}
	if closed[0].ClosedAt != 1_700_000_200 {
		t.Errorf("ClosedAt = %d, want the close's own stamp", closed[0].ClosedAt)
	}
}

// TestClosedSessionsIdentifiesEachRow pins what a history row carries, since
// that is what makes a closed session findable at all: nothing here is derivable
// from the session id the row is keyed by.
func TestClosedSessionsIdentifiesEachRow(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "gone", "Wire the relay inbox", "shell", "/wt/relay", 3, "")
	atClock(t, time.Unix(1_700_000_000, 0), func() { _ = svc.CloseSession("p1", "gone", "keep") })

	closed, _ := svc.ClosedSessions("")
	if len(closed) != 1 {
		t.Fatalf("got %d closed sessions, want 1", len(closed))
	}
	got := closed[0]
	want := ClosedSession{
		ID: "gone", ProjectID: "p1", ProjectName: "alpha", ProjectPath: "/tmp/alpha",
		Label: "Wire the relay inbox", Kind: "shell", Path: "/wt/relay",
		ClosedAt: 1_700_000_000,
	}
	if got != want {
		t.Errorf("closed session = %+v, want %+v", got, want)
	}
}

// TestClosedSessionsSkipsOpenOnes guards the list against the sessions the user
// is looking at: a history that offered to resume a live card would resume it
// against a PTY already running.
func TestClosedSessionsSkipsOpenOnes(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "live", "Session 1", "claude", "", 2, "")

	closed, err := svc.ClosedSessions("")
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	if len(closed) != 0 {
		t.Errorf("closed = %+v, want none while the only session is open", closed)
	}
}

// TestClosedSessionsIncludesClosedProjects is the case the join must not drop:
// the work a user goes looking for months later is usually in a project whose
// tab is long gone, and resuming one of its sessions is what reopens it.
func TestClosedSessionsIncludesClosedProjects(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "old", "old work", "claude", "/wt/old", 3, "")
	_ = svc.CloseSession("p1", "old", "keep")
	_ = svc.CloseProject("p1")

	closed, _ := svc.ClosedSessions("")
	if len(closed) != 1 || closed[0].ProjectName != "alpha" {
		t.Errorf("closed = %+v, want the session of the closed project, named", closed)
	}
}

// TestClosedSessionsCapsTheList pins the bound rather than deriving it from the
// constant: the cap is how many rows one call answers with, so a change to it
// has to be a deliberate edit here too.
func TestClosedSessionsCapsTheList(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	for i := range 101 {
		id := "s" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		_ = svc.AddSession("p1", id, "work", "claude", "/wt/"+id, i+3, "")
		atClock(t, time.Unix(int64(1_700_000_000+i), 0), func() {
			_ = svc.CloseSession("p1", id, "keep")
		})
	}

	closed, _ := svc.ClosedSessions("")
	if len(closed) != 100 {
		t.Errorf("got %d closed sessions, want the list capped at 100", len(closed))
	}
	// The cap keeps the newest, not the first hundred it happened to read.
	if closed[0].ClosedAt != 1_700_000_100 {
		t.Errorf("newest = %d, want the last close of the run", closed[0].ClosedAt)
	}
}

// TestReopenSessionRestoresByID covers the history list's own door: the row is
// picked, not looked up by path, and a project-root session has no path to look
// it up by in the first place.
func TestReopenSessionRestoresByID(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "root", "root work", "claude", "", 3, "")
	_ = svc.SetProviderSession("root", "conv-root")
	_ = svc.RenameSession("root", "named by hand")
	_ = svc.CloseSession("p1", "root", "keep")

	restored, err := svc.ReopenSession("root", "fresh")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession = nil, want the parked session")
	}
	if restored.ID != "fresh" || restored.Label != "named by hand" {
		t.Errorf("restored = %+v, want a fresh id keeping the chosen label", restored)
	}
	if restored.ProviderSessionID != "conv-root" {
		t.Errorf("ProviderSessionID = %q, want the conversation carried over", restored.ProviderSessionID)
	}

	// It comes back open, in its project, and stops being history.
	projects, _ := svc.LoadState()
	if len(projects) != 1 || len(projects[0].Sessions) != 2 {
		t.Fatalf("sessions after resume = %+v, want the kept one and the resumed one", projects)
	}
	closed, _ := svc.ClosedSessions("")
	if len(closed) != 0 {
		t.Errorf("closed = %+v, want the resumed row out of the history", closed)
	}
}

// TestReopenSessionKeepsThePin: the pin holds until the user takes it off, and
// a park by `lich close` — which the sidebar's withheld close button cannot
// stop — is not that. A resume that dropped it would put the card back in the
// drag order with its close affordances restored, unasked.
func TestReopenSessionKeepsThePin(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "root", "root work", "claude", "", 3, "")
	if err := svc.SetSessionPinned("root", true); err != nil {
		t.Fatalf("SetSessionPinned: %v", err)
	}
	_ = svc.CloseSession("p1", "root", "keep")

	restored, err := svc.ReopenSession("root", "fresh")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil || !restored.Pinned {
		t.Fatalf("restored = %+v, want the pin carried over", restored)
	}
	projects, _ := svc.LoadState()
	for _, s := range projects[0].Sessions {
		if s.ID == "fresh" && !s.Pinned {
			t.Errorf("reloaded session = %+v, want pinned", s)
		}
	}
}

// TestReopenSessionClearsTheCloseStamp keeps a resumed row from dating its next
// close before it happens — the stamp belongs to the close that made it, and
// carrying it over would sort the row by a close it no longer had.
func TestReopenSessionClearsTheCloseStamp(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "s1", "work", "claude", "/wt/a", 3, "")
	atClock(t, time.Unix(1_700_000_000, 0), func() { _ = svc.CloseSession("p1", "s1", "keep") })

	if _, err := svc.ReopenSession("s1", "s2"); err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	var stamp int64
	if err := svc.db.QueryRow(`SELECT closed_at FROM sessions WHERE id = ?`, "s2").Scan(&stamp); err != nil {
		t.Fatalf("read closed_at: %v", err)
	}
	if stamp != 0 {
		t.Errorf("closed_at after resume = %d, want 0 on an open row", stamp)
	}
}

// TestReopenSessionRefusesALiveOne stops the history's door from reaching a card
// on screen: resuming one would delete and reinsert the row under a new id while
// its PTY is still running against the old one.
func TestReopenSessionRefusesALiveOne(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "live", "Session 1", "claude", "", 2, "")

	restored, err := svc.ReopenSession("live", "fresh")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored != nil {
		t.Errorf("ReopenSession on an open session = %+v, want nil", restored)
	}
}

// TestReopenWorktreeSessionRefusesTheEmptyPath is PurgeWorktreeSessions' guard
// on the read side. Now that every close parks, a project's own sessions sit in
// the table with no path — and an unguarded lookup would hand one of those to a
// worktree picker that asked for a checkout.
func TestReopenWorktreeSessionRefusesTheEmptyPath(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "root", "root work", "claude", "", 3, "")
	_ = svc.CloseSession("p1", "root", "keep")

	restored, err := svc.ReopenWorktreeSession("p1", "", "fresh")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored != nil {
		t.Errorf("ReopenWorktreeSession(path: \"\") = %+v, want nil", restored)
	}
	// And the parked root session is still there for its own door.
	closed, _ := svc.ClosedSessions("")
	if len(closed) != 1 || closed[0].ID != "root" {
		t.Errorf("closed = %+v, want the root session still parked", closed)
	}
}

// TestForgetSessionRemovesOneParkedRow covers the only way out for a row whose
// checkout was removed behind lich's back — nothing else ever collects it.
func TestForgetSessionRemovesOneParkedRow(t *testing.T) {
	svc := newTestStore(t)
	var gone []string
	svc.SetSessionGone(func(id string) { gone = append(gone, id) })
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "dead", "work", "claude", "/wt/dead", 3, "")
	_ = svc.CloseSession("p1", "dead", "keep")

	if err := svc.ForgetSession("dead"); err != nil {
		t.Fatalf("ForgetSession: %v", err)
	}
	closed, _ := svc.ClosedSessions("")
	if len(closed) != 0 {
		t.Errorf("closed = %+v, want the forgotten row gone", closed)
	}
	if len(gone) != 1 || gone[0] != "dead" {
		t.Errorf("reported gone = %v, want the forgotten session once", gone)
	}
}

// TestForgetSessionRefusesALiveOne is the guard that keeps the verb apart from
// closing: a card on screen is closed, never forgotten, and an id that reached
// here by mistake must not take a running session's row with it.
func TestForgetSessionRefusesALiveOne(t *testing.T) {
	svc := newTestStore(t)
	var gone []string
	svc.SetSessionGone(func(id string) { gone = append(gone, id) })
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "live", "Session 1", "claude", "", 2, "")

	if err := svc.ForgetSession("live"); err != nil {
		t.Fatalf("ForgetSession: %v", err)
	}
	projects, _ := svc.LoadState()
	if len(projects) != 1 || len(projects[0].Sessions) != 1 {
		t.Errorf("sessions = %+v, want the live one untouched", projects)
	}
	if len(gone) != 0 {
		t.Errorf("reported gone = %v, want nothing reported for a row that stayed", gone)
	}
}

// TestForgetSessionOnAMissingRowIsNotAnError keeps the action idempotent: two
// windows can forget the same row, and the second must not raise.
func TestForgetSessionOnAMissingRowIsNotAnError(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.ForgetSession("never-existed"); err != nil {
		t.Errorf("ForgetSession on a missing row = %v, want nil", err)
	}
}

// park closes one session of project p1 at a fixed clock, so a search test can
// name what it is looking for and when it was parked in one line.
func park(t *testing.T, svc *Service, id, label string, at int64) {
	t.Helper()
	_ = svc.AddSession("p1", id, label, "claude", "/wt/"+id, 0, "")
	atClock(t, time.Unix(at, 0), func() { _ = svc.CloseSession("p1", id, "keep") })
}

// searchStore is a store with a keep-open session and three parked ones, closed
// oldest first.
func searchStore(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 1, "")
	park(t, svc, "s1", "Wire the relay inbox", 1_700_000_100)
	park(t, svc, "s2", "relay ticket lifecycle", 1_700_000_200)
	park(t, svc, "s3", "Split groups design", 1_700_000_300)
	return svc
}

func ids(closed []ClosedSession) []string {
	got := make([]string, len(closed))
	for i, c := range closed {
		got[i] = c.ID
	}
	return got
}

// TestClosedSessionsSearchesByName is the search itself: a term narrows the list
// to the rows whose name contains it, whatever their case.
func TestClosedSessionsSearchesByName(t *testing.T) {
	svc := searchStore(t)

	for _, term := range []string{"relay", "RELAY", "ReLaY"} {
		closed, err := svc.ClosedSessions(term)
		if err != nil {
			t.Fatalf("ClosedSessions(%q): %v", term, err)
		}
		// Newest first, the same order the unfiltered list is in.
		if got := ids(closed); len(got) != 2 || got[0] != "s2" || got[1] != "s1" {
			t.Errorf("ClosedSessions(%q) = %v, want [s2 s1]", term, got)
		}
	}
}

// TestClosedSessionsSearchesEveryWord pins the term's reading against the
// palette's own filter, which requires every word and not the phrase.
func TestClosedSessionsSearchesEveryWord(t *testing.T) {
	svc := searchStore(t)

	if got := ids(mustClosed(t, svc, "inbox relay")); len(got) != 1 || got[0] != "s1" {
		t.Errorf(`ClosedSessions("inbox relay") = %v, want [s1] whatever the word order`, got)
	}
	if got := ids(mustClosed(t, svc, "relay split")); len(got) != 0 {
		t.Errorf(`ClosedSessions("relay split") = %v, want none: no row carries both`, got)
	}
	// The project's name and the checkout path are part of the haystack, so a
	// term the window would have matched is not dropped before it gets there.
	if got := ids(mustClosed(t, svc, "alpha s3")); len(got) != 1 || got[0] != "s3" {
		t.Errorf(`ClosedSessions("alpha s3") = %v, want [s3] by project and path`, got)
	}
}

// TestClosedSessionsSearchEscapesWildcards is why the term is escaped: LIKE
// reads % and _ as patterns, so an unescaped name containing either would match
// rows that have nothing to do with it.
func TestClosedSessionsSearchEscapesWildcards(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 1, "")
	park(t, svc, "pct", "100% done", 1_700_000_100)
	park(t, svc, "und", "hand_off", 1_700_000_200)
	park(t, svc, "esc", `back\slash`, 1_700_000_300)
	// The decoys are what an unescaped term matches: % swallows the trailing
	// zero, _ stands in for the X, and a bare backslash is an escape sequence
	// SQLite refuses outright.
	park(t, svc, "wide", "1000 lines", 1_700_000_400)
	park(t, svc, "any", "handXoff", 1_700_000_500)

	cases := []struct {
		term string
		want string
	}{
		{"100%", "pct"},
		{"hand_off", "und"},
		{`back\slash`, "esc"},
	}
	for _, tc := range cases {
		got := ids(mustClosed(t, svc, tc.term))
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("ClosedSessions(%q) = %v, want [%s] only", tc.term, got, tc.want)
		}
	}
}

// TestClosedSessionsSearchReachesPastTheCap is the ceiling this search closed: a
// session parked further back than one page of history is still findable by
// name, which is the whole point of matching in the query.
func TestClosedSessionsSearchReachesPastTheCap(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 1, "")
	park(t, svc, "needle", "the oldest work", 1_700_000_000)
	for i := range closedSessionLimit + 5 {
		park(t, svc, fmt.Sprintf("s%d", i), "newer work", int64(1_700_001_000+i))
	}

	if got := ids(mustClosed(t, svc, "")); len(got) != closedSessionLimit {
		t.Fatalf("unfiltered list = %d rows, want %d", len(got), closedSessionLimit)
	} else if slices.Contains(got, "needle") {
		t.Fatal("the oldest row is inside the unfiltered page; the test proves nothing")
	}
	if got := ids(mustClosed(t, svc, "oldest")); len(got) != 1 || got[0] != "needle" {
		t.Errorf(`ClosedSessions("oldest") = %v, want [needle] from past the cap`, got)
	}
}

// TestClosedSessionsSearchCapsMatches pins that the cap outlives the search: a
// term matching everything is still one page of rows.
func TestClosedSessionsSearchCapsMatches(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "keep", "Session 1", "claude", "", 1, "")
	for i := range closedSessionLimit + 5 {
		park(t, svc, fmt.Sprintf("s%d", i), "work", int64(1_700_000_000+i))
	}

	closed := mustClosed(t, svc, "work")
	if len(closed) != closedSessionLimit {
		t.Fatalf("got %d matches, want the list capped at %d", len(closed), closedSessionLimit)
	}
	if closed[0].ClosedAt != int64(1_700_000_000+closedSessionLimit+4) {
		t.Errorf("newest match = %d, want the last close of the run", closed[0].ClosedAt)
	}
}

// TestEscapeLike covers the escape on its own, since the backslash has to be
// doubled before the wildcards are escaped with it.
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"plain": "plain",
		"50%":   `50\%`,
		"a_b":   `a\_b`,
		`c\d`:   `c\\d`,
		`\%_`:   `\\\%\_`,
		"":      "",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustClosed(t *testing.T, svc *Service, term string) []ClosedSession {
	t.Helper()
	closed, err := svc.ClosedSessions(term)
	if err != nil {
		t.Fatalf("ClosedSessions(%q): %v", term, err)
	}
	return closed
}
