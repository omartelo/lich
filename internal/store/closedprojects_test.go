package store

import (
	"fmt"
	"slices"
	"testing"
)

// closeProject opens a project and closes it again, which is the only way a row
// reaches the reopen list: closed_seq is stamped by the close.
func closeProject(t *testing.T, svc *Service, id, name, path string) {
	t.Helper()
	if err := svc.AddProject(id, name, path); err != nil {
		t.Fatalf("AddProject(%s): %v", id, err)
	}
	if err := svc.CloseProject(id); err != nil {
		t.Fatalf("CloseProject(%s): %v", id, err)
	}
}

// searchIDs is the reopen list for one term, reduced to what the order is
// asserted on.
func searchIDs(t *testing.T, svc *Service, term string) []string {
	t.Helper()
	recents, err := svc.RecentProjects(term)
	if err != nil {
		t.Fatalf("RecentProjects(%q): %v", term, err)
	}
	ids := make([]string, len(recents))
	for i, r := range recents {
		ids[i] = r.ID
	}
	return ids
}

// TestRecentProjectsSearchesNameAndPath is the search itself: a term narrows the
// list whatever its case, every word has to land, and the path is part of the
// haystack because that is the half of a row a name does not say.
func TestRecentProjectsSearchesNameAndPath(t *testing.T) {
	svc := newTestStore(t)
	closeProject(t, svc, "p1", "relay inbox", "/src/relay")
	closeProject(t, svc, "p2", "relay tickets", "/src/relay-tickets")
	closeProject(t, svc, "p3", "split groups", "/work/split")

	for _, term := range []string{"relay", "RELAY", "ReLaY"} {
		// Newest close first, the same order the unfiltered list is in.
		if got := searchIDs(t, svc, term); !slices.Equal(got, []string{"p2", "p1"}) {
			t.Errorf("RecentProjects(%q) = %v, want [p2 p1]", term, got)
		}
	}
	if got := searchIDs(t, svc, "inbox relay"); !slices.Equal(got, []string{"p1"}) {
		t.Errorf(`RecentProjects("inbox relay") = %v, want [p1] whatever the word order`, got)
	}
	if got := searchIDs(t, svc, "relay split"); len(got) != 0 {
		t.Errorf(`RecentProjects("relay split") = %v, want none: no row carries both`, got)
	}
	if got := searchIDs(t, svc, "work split"); !slices.Equal(got, []string{"p3"}) {
		t.Errorf(`RecentProjects("work split") = %v, want [p3] by path`, got)
	}
	if got := searchIDs(t, svc, ""); !slices.Equal(got, []string{"p3", "p2", "p1"}) {
		t.Errorf(`RecentProjects("") = %v, want the whole list newest first`, got)
	}
}

// TestRecentProjectsSearchSkipsOpenOnes pins the term against the flag: a
// matching project that is on screen is not offered for reopening.
func TestRecentProjectsSearchSkipsOpenOnes(t *testing.T) {
	svc := newTestStore(t)
	closeProject(t, svc, "p1", "relay inbox", "/src/relay")
	if err := svc.AddProject("p2", "relay tickets", "/src/relay-tickets"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	if got := searchIDs(t, svc, "relay"); !slices.Equal(got, []string{"p1"}) {
		t.Errorf(`RecentProjects("relay") = %v, want [p1]: p2 is open`, got)
	}
}

// TestRecentProjectsSearchEscapesWildcards is why the term is escaped: LIKE
// reads % and _ as patterns, so an unescaped name containing either would match
// rows that have nothing to do with it.
func TestRecentProjectsSearchEscapesWildcards(t *testing.T) {
	svc := newTestStore(t)
	closeProject(t, svc, "pct", "100% done", "/src/pct")
	closeProject(t, svc, "und", "hand_off", "/src/und")
	closeProject(t, svc, "esc", `back\slash`, "/src/esc")
	// The decoys are what an unescaped term matches: % swallows the trailing
	// zero, _ stands in for the X, and a bare backslash is an escape sequence
	// SQLite refuses outright.
	closeProject(t, svc, "wide", "1000 lines", "/src/wide")
	closeProject(t, svc, "any", "handXoff", "/src/any")

	cases := map[string]string{
		"100%":       "pct",
		"hand_off":   "und",
		`back\slash`: "esc",
	}
	for term, want := range cases {
		if got := searchIDs(t, svc, term); !slices.Equal(got, []string{want}) {
			t.Errorf("RecentProjects(%q) = %v, want [%s] only", term, got, want)
		}
	}
}

// TestRecentProjectsSearchReachesPastTheCap is the ceiling this search closed: a
// project closed further back than one page of the reopen list is still findable
// by name, which is the whole point of matching in the query.
func TestRecentProjectsSearchReachesPastTheCap(t *testing.T) {
	svc := newTestStore(t)
	closeProject(t, svc, "needle", "the oldest project", "/src/needle")
	for i := range recentLimit + 5 {
		id := fmt.Sprintf("p%d", i)
		closeProject(t, svc, id, "newer project", "/src/"+id)
	}

	unfiltered := searchIDs(t, svc, "")
	if len(unfiltered) != recentLimit {
		t.Fatalf("unfiltered list = %d rows, want %d", len(unfiltered), recentLimit)
	}
	if slices.Contains(unfiltered, "needle") {
		t.Fatal("the oldest row is inside the unfiltered page; the test proves nothing")
	}
	if got := searchIDs(t, svc, "oldest"); !slices.Equal(got, []string{"needle"}) {
		t.Errorf(`RecentProjects("oldest") = %v, want [needle] from past the cap`, got)
	}
}

// TestRecentProjectsSearchCapsMatches pins that the cap outlives the search: a
// term matching everything is still one page, newest close first.
func TestRecentProjectsSearchCapsMatches(t *testing.T) {
	svc := newTestStore(t)
	for i := range recentLimit + 5 {
		id := fmt.Sprintf("p%d", i)
		closeProject(t, svc, id, "project work", "/src/"+id)
	}

	got := searchIDs(t, svc, "work")
	if len(got) != recentLimit {
		t.Fatalf("got %d matches, want the list capped at %d", len(got), recentLimit)
	}
	if got[0] != fmt.Sprintf("p%d", recentLimit+4) {
		t.Errorf("newest match = %q, want the last project closed", got[0])
	}
}

// TestClosedProjectCountAnswersPastTheCap is what the cut is reported with: the
// count is over every match, not over the page, so the menu and the palette can
// say how many they are leaving out.
func TestClosedProjectCountAnswersPastTheCap(t *testing.T) {
	svc := newTestStore(t)
	for i := range recentLimit + 5 {
		id := fmt.Sprintf("p%d", i)
		closeProject(t, svc, id, "project work", "/src/"+id)
	}
	closeProject(t, svc, "other", "unrelated", "/src/other")
	if err := svc.AddProject("open", "project work open", "/src/open"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	cases := map[string]int{
		"":         recentLimit + 6,
		"work":     recentLimit + 5,
		"unrelate": 1,
		"nothing":  0,
	}
	for term, want := range cases {
		got, err := svc.ClosedProjectCount(term)
		if err != nil {
			t.Fatalf("ClosedProjectCount(%q): %v", term, err)
		}
		if got != want {
			t.Errorf("ClosedProjectCount(%q) = %d, want %d", term, got, want)
		}
	}
}

// TestClosedProjectCountEscapesWildcards holds the count to the same reading as
// the list: they are one search, and a term that counted more than it listed
// would report a cut that is not there.
func TestClosedProjectCountEscapesWildcards(t *testing.T) {
	svc := newTestStore(t)
	closeProject(t, svc, "pct", "100% done", "/src/pct")
	closeProject(t, svc, "wide", "1000 lines", "/src/wide")

	got, err := svc.ClosedProjectCount("100%")
	if err != nil {
		t.Fatalf("ClosedProjectCount: %v", err)
	}
	if got != 1 {
		t.Errorf(`ClosedProjectCount("100%%") = %d, want 1`, got)
	}
}
