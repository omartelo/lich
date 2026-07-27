package project

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGitMessage(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		runErr error
		want   string
	}{
		{
			"a folder that is not a repository",
			"fatal: not a git repository (or any of the parent directories): .git",
			errTest,
			"This folder is not a git repository.",
		},
		{
			"a branch that is taken",
			"fatal: a branch named 'fix/poll' already exists",
			errTest,
			"A branch or worktree with that name already exists.",
		},
		{
			"a branch checked out elsewhere",
			"fatal: 'fix/poll' is already checked out at '/data/worktrees/p/fix'",
			errTest,
			"That branch is already checked out in another worktree.",
		},
		{
			"a dirty worktree git refuses to remove",
			"fatal: '/data/worktrees/p/fix' contains modified or untracked files, use --force to delete it",
			errTest,
			"That worktree has uncommitted changes. Remove it from the sidebar to discard them.",
		},
		{
			"a remote that rejected the key",
			"git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.",
			errTest,
			"The remote refused the connection — check its ssh key or credentials.",
		},
		{
			"a remote branch that is gone",
			"fatal: couldn't find remote ref feature/old",
			errTest,
			"That branch is no longer on the remote.",
		},
		{
			"a worktree git has no registration for",
			"fatal: '/data/worktrees/p/gone' is not a working tree",
			errTest,
			"git no longer knows that worktree — it may already be gone.",
		},
		{
			"a path git does not track",
			"error: pathspec 'nope.txt' did not match any file(s) known to git",
			errTest,
			"git does not track that file.",
		},
		{
			"git missing",
			"", &exec.Error{Name: "git", Err: exec.ErrNotFound},
			"git is not installed.",
		},
		{
			// check-ref-format's own refusal: exit status, nothing on stderr.
			"a refusal with nothing on stderr",
			"", errTest,
			"git could not complete the operation. lich's log has what it reported.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitMessage(tt.stderr, tt.runErr); got != tt.want {
				t.Errorf("gitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGitMessageLeaksNothing is the rule rather than one case of it: git's own
// vocabulary never reaches the screen.
func TestGitMessageLeaksNothing(t *testing.T) {
	leaks := []string{"fatal:", "error:", "exit status", "usage:", "hint:", "--force"}
	stderrs := []string{
		"fatal: not a git repository (or any of the parent directories): .git",
		"fatal: '/data/wt' contains modified or untracked files, use --force to delete it",
		"error: pathspec 'nope.txt' did not match any file(s) known to git",
		"hint: Updates were rejected because the tip of your current branch is behind",
		"",
	}
	for _, stderr := range stderrs {
		got := strings.ToLower(gitMessage(stderr, errTest))
		for _, leak := range leaks {
			if strings.Contains(got, leak) {
				t.Errorf("gitMessage(%q) = %q, which leaks %q", stderr, got, leak)
			}
		}
	}
}

// TestRunGitMessage runs the real git against a directory that is not a
// repository: the whole path, from stderr to the sentence the screen shows.
func TestRunGitMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	_, err := runGit(t.TempDir(), "status", "--porcelain")
	if err == nil {
		t.Fatal("runGit(non-repo) = nil error, want a failure")
	}
	if want := "This folder is not a git repository."; err.Error() != want {
		t.Errorf("runGit error = %q, want %q", err, want)
	}
}
