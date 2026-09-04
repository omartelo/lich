package project

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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

	got, err := New(nil).PullRequestConflicts(clone, 7, "main", "")
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

	got, err := New(nil).PullRequestConflicts(clone, 7, "main", "")
	if err != nil {
		t.Fatalf("PullRequestConflicts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("conflicts = %v, want none — the two sides touch different files", got)
	}
}

func TestPullRequestConflictsRefusesAnUnaddressedPullRequest(t *testing.T) {
	clone, _, _ := pullRequestOrigin(t, 7)

	if _, err := New(nil).PullRequestConflicts(clone, 0, "main", ""); err == nil {
		t.Error("want an error for pull request 0 — this has no branch fallback")
	}
	if _, err := New(nil).PullRequestConflicts(clone, 7, "", ""); err == nil {
		t.Error("want an error with no base branch")
	}
}

func TestPullRequestConflictsReportsAFetchItCannotMake(t *testing.T) {
	clone, _, _ := pullRequestOrigin(t, 7)

	if _, err := New(nil).PullRequestConflicts(clone, 404, "main", ""); err == nil {
		t.Error("want an error for a pull request origin does not have")
	}
}

// A merge-tree that never answered is not a clean merge: the screen whose whole
// content is this list has to say it could not be computed. A directory that is
// no repository is the reachable stand-in for the git too old to know
// --write-tree, which exits the same way — anything but 0 or 1.
func TestMergeConflictsReportsAMergeItCannotCompute(t *testing.T) {
	if _, ok := mergeConflicts(t.TempDir(), "refs/heads/main", "refs/heads/pr"); ok {
		t.Error("want ok=false where merge-tree could not run — a clean merge and an unanswered one must not read alike")
	}
}

func TestPullRequestConflictsFetchesTheRemoteTheURLNames(t *testing.T) {
	clone, origin, originGit := pullRequestOrigin(t, 7)
	commitFile(t, origin, originGit, "a.txt", "the base branch's line\n", "base edits a")
	// The clone of a fork lich has to answer for: the pull request lives on a
	// remote nobody called origin. The URL is spelled from the origin path's own
	// last two segments, because owner/name is what the two are matched on.
	gitIn(t, clone)("remote", "rename", "origin", "upstream")
	parts := strings.Split(filepath.ToSlash(origin), "/")
	prURL := fmt.Sprintf("https://github.com/%s/%s/pull/7", parts[len(parts)-2], parts[len(parts)-1])

	got, err := New(nil).PullRequestConflicts(clone, 7, "main", prURL)
	if err != nil {
		t.Fatalf("PullRequestConflicts: %v", err)
	}
	if !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("conflicts = %v, want [a.txt] — the fetch has to reach the remote the URL names", got)
	}
}

func TestPRRemoteFallsBackToOrigin(t *testing.T) {
	clone, _, _ := pullRequestOrigin(t, 7)

	if got := prRemote(clone, ""); got != "origin" {
		t.Errorf("prRemote with no URL = %q, want origin", got)
	}
	if got := prRemote(clone, "https://github.com/nobody/else/pull/7"); got != "origin" {
		t.Errorf("prRemote for a repository no remote holds = %q, want origin", got)
	}
}

func TestRemoteRepoReadsBothSpellings(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"git@github.com:Owner/Repo.git (fetch)", "owner/repo"},
		{"https://github.com/owner/repo.git (push)", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "owner/repo"},
		{"", ""},
		{"weird", ""},
	} {
		if got := remoteRepo(tc.url); got != tc.want {
			t.Errorf("remoteRepo(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestRepoFromPRURLRefusesWhatItCannotRead(t *testing.T) {
	for _, bad := range []string{"", "https://github.com/owner", "not a url at all", "://"} {
		if got := repoFromPRURL(bad); got != "" {
			t.Errorf("repoFromPRURL(%q) = %q, want empty", bad, got)
		}
	}
}
