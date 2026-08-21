package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	git := gitIn(t, repo)
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
		got.Worktrees[0].Path != wt.Path {
		t.Errorf("Worktrees = %v, want [{resume-me %s}]", got.Worktrees, wt.Path)
	}
	// resume-me is now checked out in a worktree, so it belongs only to the
	// Worktrees group — it must not also appear in Local, where it would offer
	// to spawn a new worktree off itself.
	if !slices.Equal(got.Local, []string{"extra", "main"}) {
		t.Errorf("Local after create = %v, want [extra main] (resume-me is a worktree, not a plain base)", got.Local)
	}

	if _, err := svc.ListBranches(t.TempDir()); err == nil {
		t.Error("ListBranches(non-repo) = nil error, want error")
	}
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
	if want := canonicalPath(filepath.Join(data, "lich", "worktrees", "pid", "feat/x")); wt.Path != want {
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

// TestCreateWorktreeErrors proves invalid names, branches held by another
// checkout and pre-existing paths all fail with a message instead of
// half-creating state.
//
// A branch that merely exists is no longer among them: it used to be, back when
// every creation went through `worktree add -b`. That expectation moved to
// TestCreateWorktreeExistingBranch — the contract changed, the test followed it.
func TestCreateWorktreeErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	// Held by a checkout of its own, which is the state git still refuses: one
	// branch cannot be checked out twice.
	git("worktree", "add", "-b", "taken", filepath.Join(t.TempDir(), "taken"))

	svc := New(nil)
	// The messages are what the dialog shows, so they name the branch rather
	// than the git plumbing that rejected it (giterror.go).
	tests := []struct {
		name    string
		wtName  string
		wantSub string
	}{
		{"invalid name", "has space", `"has space" is not a name git accepts`},
		{"dotdot name", "a..b", `"a..b" is not a name git accepts`},
		{"branch checked out elsewhere", "taken", "already checked out"},
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

// TestCreateWorktreeExistingBranch replicates issue #334: a branch cut before
// the session — off an updated remote, linked to a GitHub issue — is checked
// out as it stands rather than recreated, and the base is ignored because the
// branch already started somewhere.
//
// lich handed git `worktree add -b` for every creation, and git refuses a name
// it already knows, so the only way to open a session on a prepared branch was
// to delete it first — exactly what the reporter could not do, with the issue
// branch already pushed.
func TestCreateWorktreeExistingBranch(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	addLocalOrigin(t, git)

	// The branch carries a commit no base has, so "reused" is provable rather
	// than a name that happens to match: rewinding main leaves the commit
	// reachable only from the branch.
	const branch = "issue-4-ownership"
	git("commit", "--allow-empty", "-m", "prepared work")
	git("branch", branch)
	prepared := strings.TrimSpace(git("rev-parse", branch))
	git("reset", "--hard", "HEAD~1")

	// The reporter's call, verbatim: the prepared branch as the name, the remote
	// it was cut from as the base.
	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", branch, "origin/main", true)
	if err != nil {
		t.Fatalf("CreateWorktree(existing branch): %v", err)
	}
	if wt.Name != branch {
		t.Errorf("Name = %q, want %q", wt.Name, branch)
	}

	head, err := runGit(wt.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if got := strings.TrimSpace(head); got != prepared {
		t.Errorf("worktree HEAD = %s, want the prepared branch tip %s — the base was not ignored", got, prepared)
	}
	on, err := runGit(wt.Path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse --abbrev-ref HEAD: %v", err)
	}
	if got := strings.TrimSpace(on); got != branch {
		t.Errorf("worktree is on %q, want %q", got, branch)
	}

	// git has to own it too, or nothing can remove it later.
	if !slices.ContainsFunc(parseCheckouts(git("worktree", "list", "--porcelain")), func(c Worktree) bool {
		return c.Name == branch
	}) {
		t.Errorf("the branch is not registered as a worktree: %s", git("worktree", "list"))
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

// scriptedGH fakes the gh CLI for the PR checkout flow: `pr view` answers with
// headJSON, and `pr checkout` either fails with checkoutErr or does what gh
// would — create the head branch in the worktree it was called in. It records
// the directory each call ran in, which is the whole point of the flow (the
// checkout must happen inside the new worktree, not the project).
type scriptedGH struct {
	headJSON    string
	checkoutErr error
	// lockOnFail locks the worktree before failing. git refuses to remove a
	// locked worktree, which is how the cleanup itself is made to fail.
	lockOnFail bool
	// token is what `gh auth token` answers with, so a test can follow which
	// account each call in the flow ran as.
	token         string
	viewDir       string
	viewToken     string
	checkoutDir   string
	checkoutToken string
	calls         []string
}

func (g *scriptedGH) run(_ time.Duration, dir, token string, args ...string) ([]byte, error) {
	g.calls = append(g.calls, strings.Join(args, " "))
	if len(args) > 1 && args[0] == "auth" && args[1] == "token" {
		return []byte(g.token + "\n"), nil
	}
	if len(args) > 1 && args[1] == "checkout" {
		g.checkoutDir, g.checkoutToken = dir, token
		if g.checkoutErr != nil {
			if g.lockOnFail {
				if out, err := exec.Command("git", "-C", dir, "worktree", "lock", dir).CombinedOutput(); err != nil {
					return nil, fmt.Errorf("fake lock: %v (%s)", err, out)
				}
			}
			return nil, g.checkoutErr
		}
		var head prHead
		if err := json.Unmarshal([]byte(g.headJSON), &head); err != nil {
			return nil, err
		}
		if out, err := exec.Command("git", "-C", dir, "checkout", "-b", head.RefName).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("fake checkout: %v (%s)", err, out)
		}
		return nil, nil
	}
	g.viewDir, g.viewToken = dir, token
	return []byte(g.headJSON), nil
}

func TestCreateWorktreeFromPR(t *testing.T) {
	t.Run("checks the head branch out into its own worktree", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		repo, git := initRepo(t)
		gh := &scriptedGH{headJSON: `{"headRefName":"fix/poll","isCrossRepository":false}`}
		svc := &Service{runner: gh.run}

		wt, err := svc.CreateWorktreeFromPR(repo, "proj", 105)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := canonicalPath(filepath.Join(data, "lich", "worktrees", "proj", "fix/poll"))
		if wt.Path != want || wt.Name != "fix/poll" {
			t.Errorf("worktree = %+v, want name fix/poll at %s", wt, want)
		}
		// The checkout has to run inside the new worktree: run in the project it
		// would move the project's own HEAD instead.
		if canonicalPath(gh.checkoutDir) != wt.Path {
			t.Errorf("gh pr checkout ran in %q, want %q", gh.checkoutDir, wt.Path)
		}
		if gh.viewDir != repo {
			t.Errorf("gh pr view ran in %q, want %q", gh.viewDir, repo)
		}
		out, err := exec.Command("git", "-C", wt.Path, "symbolic-ref", "--short", "HEAD").Output()
		if err != nil || strings.TrimSpace(string(out)) != "fix/poll" {
			t.Errorf("worktree is not on the head branch: %q (%v)", out, err)
		}
		// Compared through the same canonical form the product uses: git lists
		// a worktree by its resolved path and with forward slashes on Windows,
		// which is never the literal string filepath.Join built.
		registered := false
		for _, listed := range parseCheckouts(git("worktree", "list", "--porcelain")) {
			registered = registered || listed.Path == wt.Path
		}
		if !registered {
			t.Errorf("the worktree is not registered with git: %s", git("worktree", "list"))
		}
	})

	// The checkout runs inside a worktree the store has never seen — it has no
	// session row until the frontend opens one — so resolving the account from
	// the directory gh runs in would answer "none" and fall back to gh's active
	// account, failing on exactly the repositories the picker exists for.
	t.Run("the checkout runs as the project's account", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		repo, _ := initRepo(t)
		gh := &scriptedGH{
			headJSON: `{"headRefName":"fix/account","isCrossRepository":false}`,
			token:    "gho_project",
		}
		svc := &Service{runner: gh.run}
		// Only the project directory names an account, as the store would.
		svc.SetAccounts(func(dir string) string {
			if dir == repo {
				return "marcelo-filho_snk"
			}
			return ""
		})

		if _, err := svc.CreateWorktreeFromPR(repo, "proj", 105); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gh.checkoutToken != "gho_project" {
			t.Errorf("gh pr checkout ran with token %q, want the project's", gh.checkoutToken)
		}
		if gh.viewToken != "gho_project" {
			t.Errorf("gh pr view ran with token %q, want the project's", gh.viewToken)
		}
	})

	t.Run("a fork pull request with no maintainer edits is refused before anything is created", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		repo, _ := initRepo(t)
		gh := &scriptedGH{headJSON: `{"headRefName":"patch-1","isCrossRepository":true,"maintainerCanModify":false}`}

		_, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 103)
		if err == nil || !strings.Contains(err.Error(), "fork") {
			t.Fatalf("want a fork refusal, got %v", err)
		}
		if gh.checkoutDir != "" {
			t.Error("gh pr checkout ran for a fork pull request")
		}
	})

	// The permission is the whole gate: with it on, the fork's branch takes a
	// push back and the pull request is workable like any other.
	t.Run("a fork pull request that allows maintainer edits is checked out", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		repo, _ := initRepo(t)
		gh := &scriptedGH{headJSON: `{"headRefName":"patch-1","isCrossRepository":true,"maintainerCanModify":true}`}

		wt, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 103)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if canonicalPath(gh.checkoutDir) != wt.Path {
			t.Errorf("gh pr checkout ran in %q, want %q", gh.checkoutDir, wt.Path)
		}
	})

	t.Run("a failed checkout leaves no worktree behind", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		repo, git := initRepo(t)
		gh := &scriptedGH{
			headJSON:    `{"headRefName":"fix/poll","isCrossRepository":false}`,
			checkoutErr: errors.New("fatal: 'fix/poll' is already checked out"),
		}

		_, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 105)
		if err == nil || !strings.Contains(err.Error(), "already checked out") {
			t.Fatalf("want gh's own refusal, got %v", err)
		}
		wtPath := filepath.Join(data, "lich", "worktrees", "proj", "fix/poll")
		if _, statErr := os.Stat(wtPath); statErr == nil {
			t.Error("the failed checkout left its directory behind")
		}
		// A husk would also stay registered and block the next attempt.
		if strings.Contains(git("worktree", "list"), "fix/poll") {
			t.Error("the failed checkout stayed registered with git")
		}
	})

	// The cleanup is best-effort; what it must never do is bury the reason the
	// checkout failed under the reason the cleanup failed.
	t.Run("a cleanup that fails too still names gh's own refusal", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		repo, _ := initRepo(t)
		gh := &scriptedGH{
			headJSON:    `{"headRefName":"fix/poll","isCrossRepository":false}`,
			checkoutErr: errors.New("fatal: could not fetch the head ref"),
			lockOnFail:  true,
		}

		_, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 105)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "could not fetch the head ref") {
			t.Errorf("gh's own cause was lost: %q", err)
		}
		if !strings.Contains(err.Error(), "could not be removed") {
			t.Errorf("the failed cleanup went unreported: %q", err)
		}
	})

	t.Run("a number that cannot name a pull request never reaches gh", func(t *testing.T) {
		gh := &scriptedGH{}
		if _, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(t.TempDir(), "proj", 0); err == nil {
			t.Error("expected an error for pull request 0")
		}
		if len(gh.calls) != 0 {
			t.Errorf("gh was called %v, want no calls", gh.calls)
		}
	})

	t.Run("an occupied path is refused", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		repo, _ := initRepo(t)
		wtPath := filepath.Join(data, "lich", "worktrees", "proj", "fix/poll")
		if err := os.MkdirAll(wtPath, 0o755); err != nil {
			t.Fatal(err)
		}
		gh := &scriptedGH{headJSON: `{"headRefName":"fix/poll","isCrossRepository":false}`}

		_, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 105)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("want an already-exists refusal, got %v", err)
		}
	})
}

// TestPullRequestHead covers what gh can answer with that is not a head branch:
// the flow has to stop at the lookup rather than build a worktree named "".
func TestPullRequestHead(t *testing.T) {
	tests := []struct {
		name string
		gh   *scriptedGH
		want string
	}{
		// The decoder's own text names Go types, so the flow answers with the
		// screen's sentence instead (gherror.go).
		{"payload gh could not have meant", &scriptedGH{headJSON: `{not json`}, "could not be read"},
		{"a pull request with no head branch", &scriptedGH{headJSON: `{}`}, "no head branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&Service{runner: tt.gh.run}).pullRequestHead("/repo", 7)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error naming %q, got %v", tt.want, err)
			}
		})
	}
}

// TestCreateWorktreeFromPRRejectsRefName proves the branch name reaches git's
// own check-ref-format before it is joined into a path: gh names the branch, so
// nothing else stands between a hostile head ref and the worktrees directory.
func TestCreateWorktreeFromPRRejectsRefName(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	repo, _ := initRepo(t)
	gh := &scriptedGH{headJSON: `{"headRefName":"../../escaped","isCrossRepository":false}`}

	if _, err := (&Service{runner: gh.run}).CreateWorktreeFromPR(repo, "proj", 105); err == nil {
		t.Fatal("expected check-ref-format to refuse the head branch")
	}
	if gh.checkoutDir != "" {
		t.Error("gh pr checkout ran for a name git had already refused")
	}
	if _, err := os.Stat(filepath.Join(data, "lich", "worktrees")); err == nil {
		t.Error("a refused name still created the worktrees directory")
	}
}

// TestCheckoutPathsSurviveASymlink reproduces on any platform what macOS does
// to every path under /var and Windows to a short name: git reports a checkout
// by its resolved path while lich builds one by joining names, and the two
// spellings have to still name one worktree. Without it the session opened on a
// worktree is never recognised as living in that worktree.
func TestCheckoutPathsSurviveASymlink(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "data-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("XDG_DATA_HOME", link)
	repo, _ := initRepo(t)
	svc := &Service{}

	wt, err := svc.CreateWorktree(repo, "proj", "feature", "main", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkouts, err := svc.ListCheckouts(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.ContainsFunc(checkouts, func(c Worktree) bool { return c.Path == wt.Path }) {
		t.Errorf("CreateWorktree returned %q, which is in none of %+v", wt.Path, checkouts)
	}
}

// TestListCheckouts proves the main checkout is included — the omission that
// let "Open in Session" try to check out a branch the project itself held.
func TestListCheckouts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := initRepo(t)
	svc := &Service{}

	t.Run("the project's own checkout counts", func(t *testing.T) {
		checkouts, err := svc.ListCheckouts(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(checkouts) != 1 || checkouts[0].Name != "main" {
			t.Fatalf("want the main checkout on main, got %+v", checkouts)
		}
		// ListBranches drops it on purpose; the two must not be confused again.
		branches, err := svc.ListBranches(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(branches.Worktrees) != 0 {
			t.Errorf("ListBranches should still omit the main checkout, got %+v", branches.Worktrees)
		}
	})

	t.Run("linked worktrees come along", func(t *testing.T) {
		wt, err := svc.CreateWorktree(repo, "proj", "feature", "main", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkouts, err := svc.ListCheckouts(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var names []string
		for _, c := range checkouts {
			names = append(names, c.Name)
		}
		slices.Sort(names)
		if !slices.Equal(names, []string{"feature", "main"}) {
			t.Errorf("checkouts = %v, want [feature main]", names)
		}
		if !slices.ContainsFunc(checkouts, func(c Worktree) bool { return c.Path == wt.Path }) {
			t.Errorf("the linked worktree's path is missing from %+v", checkouts)
		}
	})

	// The caller's own directory comes back spelled the way it was passed in.
	// git answers with the resolved path, which on macOS and Windows is a
	// different string for the same directory — and the caller compares what it
	// gets against the project path it already holds.
	t.Run("the caller's own path is handed back unchanged", func(t *testing.T) {
		checkouts, err := svc.ListCheckouts(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.ContainsFunc(checkouts, func(c Worktree) bool { return c.Path == repo }) {
			t.Errorf("the project's own path is missing from %+v (asked as %q)", checkouts, repo)
		}
	})

	t.Run("a detached checkout holds no branch", func(t *testing.T) {
		git("worktree", "add", "--detach", filepath.Join(t.TempDir(), "loose"))
		checkouts, err := svc.ListCheckouts(repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, c := range checkouts {
			if c.Name == "" || strings.HasSuffix(c.Path, "loose") {
				t.Errorf("a detached entry leaked in: %+v", c)
			}
		}
	})
}
