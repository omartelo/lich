package project

import (
	"fmt"
	"slices"
	"testing"
)

// pullRequestOrigin builds what a pull request looks like from a clone's side: a
// branch that exists on origin alone, published under GitHub's own
// refs/pull/<n>/head and editing a.txt. It hands back the origin so the test can
// move the base branch itself — that is the half deciding whether the merge
// conflicts, and none of it is in the clone until the fetch under test runs.
func pullRequestOrigin(t *testing.T, number int) (clone, origin string, originGit func(args ...string) string) {
	t.Helper()
	clone, _, originGit = initClone(t)
	origin = originGit("rev-parse", "--show-toplevel")
	originGit("checkout", "--quiet", "-b", "pr")
	commitFile(t, origin, originGit, "a.txt", "the pull request's line\n", "pr edits a")
	originGit("checkout", "--quiet", "main")
	originGit("update-ref", fmt.Sprintf("refs/pull/%d/head", number), "refs/heads/pr")
	return clone, origin, originGit
}

func TestPullRequestConflictsNamesTheFiles(t *testing.T) {
	clone, origin, originGit := pullRequestOrigin(t, 7)
	commitFile(t, origin, originGit, "a.txt", "the base branch's line\n", "base edits a")

	got, err := New(nil).PullRequestConflicts(clone, 7, "main")
	if err != nil {
		t.Fatalf("PullRequestConflicts: %v", err)
	}
	if !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("conflicts = %v, want [a.txt] — both sides rewrote that line", got)
	}
}

func TestPullRequestConflictsNoneWhenTheMergeIsClean(t *testing.T) {
	clone, origin, originGit := pullRequestOrigin(t, 7)
	commitFile(t, origin, originGit, "b.txt", "elsewhere\n", "base adds b")

	got, err := New(nil).PullRequestConflicts(clone, 7, "main")
	if err != nil {
		t.Fatalf("PullRequestConflicts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("conflicts = %v, want none — the two sides touch different files", got)
	}
}

func TestPullRequestConflictsRefusesAnUnaddressedPullRequest(t *testing.T) {
	clone, _, _ := pullRequestOrigin(t, 7)

	if _, err := New(nil).PullRequestConflicts(clone, 0, "main"); err == nil {
		t.Error("want an error for pull request 0 — this has no branch fallback")
	}
	if _, err := New(nil).PullRequestConflicts(clone, 7, ""); err == nil {
		t.Error("want an error with no base branch")
	}
}

func TestPullRequestConflictsReportsAFetchItCannotMake(t *testing.T) {
	clone, _, _ := pullRequestOrigin(t, 7)

	if _, err := New(nil).PullRequestConflicts(clone, 404, "main"); err == nil {
		t.Error("want an error for a pull request origin does not have")
	}
}
