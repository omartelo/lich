package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// initRepo creates a git repository with one commit on branch main and returns
// its path plus a helper that runs git inside it. Skips the test when git is
// unavailable.
func initRepo(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
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
	// Byte-exact file content is part of the assertions (DiscardFile restores
	// HEAD bytes); pin autocrlf so neither Git for Windows' default nor the
	// machine's global config rewrites line endings on checkout.
	git("config", "core.autocrlf", "false")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-m", "init")
	return repo, git
}

// addLocalOrigin wires repo up to a second local repository acting as origin,
// so remote-branch flows (fetch, --track) run without a network.
func addLocalOrigin(t *testing.T, git func(args ...string) string) {
	t.Helper()
	origin, originGit := initRepo(t)
	originGit("branch", "feature")
	git("remote", "add", "origin", origin)
	git("fetch", "origin")
	git("remote", "set-head", "origin", "main")
}

// TestListBranches proves local branches, remote branches (minus origin/HEAD)
// and existing worktrees are all listed, and that a non-repo errors.
func TestListBranches(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	git("branch", "extra")
	addLocalOrigin(t, git)

	svc := New(nil)
	got, err := svc.ListBranches(repo)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	if !slices.Equal(got.Local, []string{"extra", "main"}) {
		t.Errorf("Local = %v, want [extra main]", got.Local)
	}
	if !slices.Equal(got.Remote, []string{"origin/feature", "origin/main"}) {
		t.Errorf("Remote = %v, want [origin/feature origin/main]", got.Remote)
	}
	if len(got.Worktrees) != 0 {
		t.Errorf("Worktrees = %v, want empty (main worktree is skipped)", got.Worktrees)
	}

	wt, err := svc.CreateWorktree(repo, "pid", "resume-me", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	got, err = svc.ListBranches(repo)
	if err != nil {
		t.Fatalf("ListBranches after create: %v", err)
	}
	if len(got.Worktrees) != 1 {
		t.Fatalf("Worktrees = %v, want one entry", got.Worktrees)
	}
	if got.Worktrees[0].Name != "resume-me" ||
		canonPath(t, got.Worktrees[0].Path) != canonPath(t, wt.Path) {
		t.Errorf("Worktrees = %v, want [{resume-me %s}]", got.Worktrees, wt.Path)
	}

	if _, err := svc.ListBranches(t.TempDir()); err == nil {
		t.Error("ListBranches(non-repo) = nil error, want error")
	}
}

// canonPath resolves platform aliasing — Windows 8.3 short names (the CI
// runner's TEMP), symlinked temp dirs — so a path reported by git and one
// built by Go compare equal when they name the same directory.
func canonPath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return resolved
}

// TestParseWorktrees proves the porcelain parser skips the main worktree and
// bare/detached entries and keeps linked worktrees on a branch.
func TestParseWorktrees(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []Worktree
	}{
		{"main only", "worktree /repo\nHEAD abc\nbranch refs/heads/main\n", []Worktree{}},
		{
			"main plus linked",
			"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /wt/feat\nHEAD def\nbranch refs/heads/feat\n",
			// The parser folds paths into the platform's native form.
			[]Worktree{{Name: "feat", Path: filepath.FromSlash("/wt/feat")}},
		},
		{
			"detached and bare skipped",
			"worktree /repo\nbare\n\n" +
				"worktree /wt/pin\nHEAD abc\ndetached\n\n" +
				"worktree /wt/ok\nHEAD def\nbranch refs/heads/ok\n",
			[]Worktree{{Name: "ok", Path: filepath.FromSlash("/wt/ok")}},
		},
		{"empty", "", []Worktree{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWorktrees(tt.out); !slices.Equal(got, tt.want) {
				t.Errorf("parseWorktrees() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCreateWorktree proves the happy path: checkout under the data dir, new
// branch checked out, and a random name when none is given.
func TestCreateWorktree(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	repo, _ := initRepo(t)

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "feat/x", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if want := filepath.Join(data, "lich", "worktrees", "pid", "feat/x"); wt.Path != want {
		t.Errorf("Path = %q, want %q", wt.Path, want)
	}
	if got := svc.Branch(wt.Path); got != "feat/x" {
		t.Errorf("Branch(worktree) = %q, want feat/x", got)
	}

	random, err := svc.CreateWorktree(repo, "pid", "", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree(random name): %v", err)
	}
	if random.Name == "" || strings.ContainsAny(random.Name, " /") {
		t.Errorf("random Name = %q, want non-empty adjective-noun", random.Name)
	}
	if _, err := os.Stat(random.Path); err != nil {
		t.Errorf("random worktree dir missing: %v", err)
	}
}

// TestCreateWorktreeErrors proves invalid names, duplicate branches and
// pre-existing paths all fail with a message instead of half-creating state.
func TestCreateWorktreeErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	git("branch", "taken")

	svc := New(nil)
	tests := []struct {
		name    string
		wtName  string
		wantSub string
	}{
		{"invalid name", "has space", "check-ref-format"},
		{"dotdot name", "a..b", "check-ref-format"},
		{"duplicate branch", "taken", "taken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateWorktree(repo, "pid", tt.wtName, "main", false)
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("CreateWorktree(%q) error = %v, want containing %q", tt.wtName, err, tt.wantSub)
			}
		})
	}

	// A leftover directory at the target path (e.g. from a crash git already
	// pruned) blocks creation with a clear message.
	leftover := filepath.Join(os.Getenv("XDG_DATA_HOME"), "lich", "worktrees", "pid", "occupied")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateWorktree(repo, "pid", "occupied", "main", false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("CreateWorktree(existing path) error = %v, want 'already exists'", err)
	}
}

// TestCreateWorktreeRemoteBase proves a remote base is fetched and the new
// branch tracks it.
func TestCreateWorktreeRemoteBase(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	addLocalOrigin(t, git)

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "from-remote", "origin/feature", true)
	if err != nil {
		t.Fatalf("CreateWorktree(remote base): %v", err)
	}
	upstream, err := runGit(wt.Path, "rev-parse", "--abbrev-ref", "@{u}")
	if err != nil {
		t.Fatalf("rev-parse @{u}: %v", err)
	}
	if got := strings.TrimSpace(upstream); got != "origin/feature" {
		t.Errorf("upstream = %q, want origin/feature", got)
	}
}

// TestRemoveWorktree proves removal deletes a clean worktree, refuses a dirty
// one without force (leaving it on disk), and deletes it with force.
func TestRemoveWorktree(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, _ := initRepo(t)

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "gone", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if err := svc.RemoveWorktree(repo, wt.Path, false); err != nil {
		t.Fatalf("RemoveWorktree(clean): %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after remove")
	}

	dirty, err := svc.CreateWorktree(repo, "pid", "dirty", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirty.Path, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveWorktree(repo, dirty.Path, false); err == nil {
		t.Error("RemoveWorktree(dirty) = nil error, want refusal")
	}
	if _, err := os.Stat(dirty.Path); err != nil {
		t.Errorf("dirty worktree should remain on disk: %v", err)
	}
	if err := svc.RemoveWorktree(repo, dirty.Path, true); err != nil {
		t.Fatalf("RemoveWorktree(dirty, force): %v", err)
	}
	if _, err := os.Stat(dirty.Path); !os.IsNotExist(err) {
		t.Errorf("dirty worktree still exists after forced remove")
	}
}

// TestWorktreeDirty proves a fresh worktree reads clean, both modified and
// untracked files read dirty, and a non-repo path errors.
func TestWorktreeDirty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, _ := initRepo(t)

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "wt", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if dirty, err := svc.WorktreeDirty(wt.Path); err != nil || dirty {
		t.Errorf("WorktreeDirty(clean) = %v, %v; want false, nil", dirty, err)
	}

	if err := os.WriteFile(filepath.Join(wt.Path, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := svc.WorktreeDirty(wt.Path); err != nil || !dirty {
		t.Errorf("WorktreeDirty(untracked file) = %v, %v; want true, nil", dirty, err)
	}

	if err := os.Remove(filepath.Join(wt.Path, "new.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt.Path, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty, err := svc.WorktreeDirty(wt.Path); err != nil || !dirty {
		t.Errorf("WorktreeDirty(modified file) = %v, %v; want true, nil", dirty, err)
	}

	if _, err := svc.WorktreeDirty(t.TempDir()); err == nil {
		t.Error("WorktreeDirty(non-repo) = nil error, want error")
	}
}
