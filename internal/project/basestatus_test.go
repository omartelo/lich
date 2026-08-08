package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// gitIn returns a helper running git inside dir with a fixed identity, failing
// the test on any non-zero exit.
func gitIn(t *testing.T, dir string) func(args ...string) string {
	t.Helper()
	return func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
}

// initClone builds what the base-branch readout needs and nothing else has: an
// origin repository and a real clone of it, so refs/remotes/origin/HEAD exists
// the way git itself writes it. Committing through originGit and fetching is how
// a test makes the clone fall behind.
func initClone(t *testing.T) (clone string, cloneGit, originGit func(args ...string) string) {
	t.Helper()
	origin, originGit := initRepo(t)
	clone = filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", "--quiet", origin, clone).CombinedOutput(); err != nil {
		t.Skipf("git clone unavailable: %v (%s)", err, out)
	}
	cloneGit = gitIn(t, clone)
	cloneGit("config", "core.autocrlf", "false")
	return clone, cloneGit, originGit
}

// noFetch books the repository's fetch slot before the readout can, so the test
// never spawns the background fetch: it outlives the assertions and would race
// the temp directory's cleanup.
func noFetch(t *testing.T, s *Service, git func(args ...string) string) {
	t.Helper()
	s.bases.claimFetch(git("rev-parse", "--path-format=absolute", "--git-common-dir"), time.Now())
}

// commitFile writes content and commits it, so a test says what changed rather
// than how git is driven.
func commitFile(t *testing.T, dir string, git func(args ...string) string, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", name)
	git("commit", "-m", message)
}

func TestBaseStatusNoRemote(t *testing.T) {
	repo, _ := initRepo(t)
	if got := New(nil).BaseStatus(repo); got != nil {
		t.Fatalf("want nil for a repository with no origin, got %+v", got)
	}
}

func TestBaseStatusNotARepository(t *testing.T) {
	if got := New(nil).BaseStatus(t.TempDir()); got != nil {
		t.Fatalf("want nil outside a repository, got %+v", got)
	}
}

func TestBaseStatusUpToDate(t *testing.T) {
	clone, cloneGit, _ := initClone(t)
	cloneGit("checkout", "-b", "feature")
	commitFile(t, clone, cloneGit, "b.txt", "two\n", "add b")

	svc := New(nil)
	noFetch(t, svc, cloneGit)
	got := svc.BaseStatus(clone)
	if got == nil {
		t.Fatal("want a readout for a clone on a feature branch, got nil")
	}
	if got.Base != "origin/main" {
		t.Errorf("base = %q, want origin/main", got.Base)
	}
	if got.Behind != 0 {
		t.Errorf("behind = %d, want 0", got.Behind)
	}
	if len(got.Conflicts) != 0 {
		t.Errorf("conflicts = %v, want none", got.Conflicts)
	}
}

func TestBaseStatusBehindWithoutConflict(t *testing.T) {
	clone, cloneGit, originGit := initClone(t)
	cloneGit("checkout", "-b", "feature")
	commitFile(t, clone, cloneGit, "b.txt", "two\n", "add b")
	origin := originGit("rev-parse", "--show-toplevel")
	commitFile(t, origin, originGit, "c.txt", "three\n", "add c")
	cloneGit("fetch", "origin")

	svc := New(nil)
	noFetch(t, svc, cloneGit)
	got := svc.BaseStatus(clone)
	if got == nil {
		t.Fatal("want a readout, got nil")
	}
	if got.Behind != 1 {
		t.Errorf("behind = %d, want 1", got.Behind)
	}
	if len(got.Conflicts) != 0 {
		t.Errorf("conflicts = %v, want none — the two commits touch different files", got.Conflicts)
	}
}

func TestBaseStatusConflict(t *testing.T) {
	clone, cloneGit, originGit := initClone(t)
	cloneGit("checkout", "-b", "feature")
	commitFile(t, clone, cloneGit, "a.txt", "mine\n", "edit a")
	origin := originGit("rev-parse", "--show-toplevel")
	commitFile(t, origin, originGit, "a.txt", "theirs\n", "edit a")
	cloneGit("fetch", "origin")

	svc := New(nil)
	noFetch(t, svc, cloneGit)
	got := svc.BaseStatus(clone)
	if got == nil {
		t.Fatal("want a readout, got nil")
	}
	if !slices.Equal(got.Conflicts, []string{"a.txt"}) {
		t.Errorf("conflicts = %v, want [a.txt]", got.Conflicts)
	}
}

// A memoised answer must not outlive either commit it was computed from — a
// stale one pins the badge to a state the checkout has left, which is the only
// way this cache can be wrong. Both keys are moved, one at a time.
func TestBaseStatusRecomputes(t *testing.T) {
	clone, cloneGit, originGit := initClone(t)
	cloneGit("checkout", "-b", "feature")
	commitFile(t, clone, cloneGit, "b.txt", "two\n", "add b")

	svc := New(nil)
	noFetch(t, svc, cloneGit)
	if got := svc.BaseStatus(clone); got == nil || got.Behind != 0 {
		t.Fatalf("first read = %+v, want behind 0", got)
	}

	origin := originGit("rev-parse", "--show-toplevel")
	commitFile(t, origin, originGit, "a.txt", "theirs\n", "edit a")
	cloneGit("fetch", "origin")
	if got := svc.BaseStatus(clone); got == nil || got.Behind != 1 {
		t.Fatalf("read after the base moved = %+v, want behind 1", got)
	}

	commitFile(t, clone, cloneGit, "a.txt", "mine\n", "edit a")
	got := svc.BaseStatus(clone)
	if got == nil || !slices.Equal(got.Conflicts, []string{"a.txt"}) {
		t.Fatalf("read after HEAD moved = %+v, want a conflict on a.txt", got)
	}
}

// merge-tree's -z output, pinned as the bytes git emits rather than rebuilt from
// the parser's own idea of them: the tree OID, the conflicted paths, an empty
// entry, then the informational messages that must not be read as paths.
func TestConflictPaths(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "clean merge is the tree alone",
			out:  "e5a3cdbec6185600f014dc64b288e05a227d5ac3\x00",
		},
		{
			name: "one conflicted path, messages behind the empty entry",
			out: "e5a3cdbec6185600f014dc64b288e05a227d5ac3\x00f.txt\x00\x00" +
				"1\x00f.txt\x00Auto-merging\x00Auto-merging f.txt\n\x00",
			want: []string{"f.txt"},
		},
		{
			name: "two conflicted paths",
			out:  "abc\x00a.txt\x00dir/b.txt\x00\x001\x00a.txt\x00CONFLICT (content)\x00",
			want: []string{"a.txt", "dir/b.txt"},
		},
		{
			name: "empty output",
			out:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := conflictPaths(tt.out); !slices.Equal(got, tt.want) {
				t.Errorf("conflictPaths = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimFetchThrottles(t *testing.T) {
	var cache baseCache
	now := time.Now()
	if !cache.claimFetch("/repo/.git", now) {
		t.Fatal("first claim refused")
	}
	if cache.claimFetch("/repo/.git", now.Add(baseFetchInterval-time.Second)) {
		t.Error("claim granted inside the interval")
	}
	if !cache.claimFetch("/repo/.git", now.Add(baseFetchInterval)) {
		t.Error("claim refused once the interval elapsed")
	}
	// A second repository has its own slot: one busy checkout must not stop
	// another repository's refs from ever moving.
	if !cache.claimFetch("/other/.git", now) {
		t.Error("claim refused for a different repository")
	}
}
