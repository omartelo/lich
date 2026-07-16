package project

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is a git worktree checkout: the branch it holds and its path.
type Worktree struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Branches groups everything the base-branch picker offers: local and remote
// branch names plus already-existing worktrees, so a closed-but-kept worktree
// can be resumed instead of recreated.
type Branches struct {
	Local     []string   `json:"local"`
	Remote    []string   `json:"remote"` // "origin/main" form
	Worktrees []Worktree `json:"worktrees"`
}

// runGit runs git -C dir args... and returns stdout. Unlike Branch/Diff, which
// deliberately swallow errors for polling, failures here carry git's stderr in
// the message so the frontend can show it verbatim.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return string(out), nil
}

// ListBranches returns the repository's local and remote branches and its
// existing worktrees. A path that is not a git repository yields an error.
func (s *Service) ListBranches(path string) (Branches, error) {
	branches := Branches{Local: []string{}, Remote: []string{}, Worktrees: []Worktree{}}

	local, err := runGit(path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return branches, err
	}
	branches.Local = append(branches.Local, splitLines(local)...)

	// Full refnames, not :short — the short form of refs/remotes/origin/HEAD is
	// just "origin", which would dodge a suffix filter.
	remote, err := runGit(path, "for-each-ref", "--format=%(refname)", "refs/remotes")
	if err != nil {
		return branches, err
	}
	for _, name := range splitLines(remote) {
		// origin/HEAD is a symbolic pointer, not a branch to base work on.
		if !strings.HasSuffix(name, "/HEAD") {
			branches.Remote = append(branches.Remote, strings.TrimPrefix(name, "refs/remotes/"))
		}
	}

	list, err := runGit(path, "worktree", "list", "--porcelain")
	if err != nil {
		return branches, err
	}
	branches.Worktrees = append(branches.Worktrees, parseWorktrees(list)...)
	return branches, nil
}

// parseWorktrees reads `git worktree list --porcelain` output: blank-line
// separated blocks of "worktree <path>" / "branch refs/heads/<name>" lines. The
// first block is always the main worktree and is skipped, as are bare and
// detached entries — only linked worktrees on a branch can host a session.
func parseWorktrees(out string) []Worktree {
	worktrees := []Worktree{}
	first := true
	for block := range strings.SplitSeq(strings.TrimSpace(out), "\n\n") {
		wt, ok := parseWorktreeBlock(block)
		if first {
			first = false
			continue
		}
		if ok {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees
}

// parseWorktreeBlock extracts one worktree entry; ok is false for bare or
// detached entries and malformed blocks.
func parseWorktreeBlock(block string) (Worktree, bool) {
	var wt Worktree
	for line := range strings.SplitSeq(block, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			// git prints forward slashes even on Windows; Clean folds the
			// path into the platform's native form so it compares equal to
			// the paths CreateWorktree builds with filepath.Join.
			if p := strings.TrimPrefix(line, "worktree "); p != "" {
				wt.Path = filepath.Clean(p)
			}
		case strings.HasPrefix(line, "branch refs/heads/"):
			wt.Name = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "bare" || line == "detached":
			return Worktree{}, false
		}
	}
	return wt, wt.Path != "" && wt.Name != ""
}

// CreateWorktree creates a git worktree named name (random when empty) under
// the app data dir, branching off base. A remote base is fetched first and the
// new branch tracks it. The worktree is verified usable before returning, so a
// success here means a session can be opened at Path right away.
func (s *Service) CreateWorktree(projectPath, projectID, name, base string, baseIsRemote bool) (*Worktree, error) {
	if name == "" {
		name = randomWorktreeName(func(n string) bool { return branchExists(projectPath, n) })
	}
	// check-ref-format is the authority on valid names; it also rejects "..",
	// which keeps the Join below free of path traversal.
	if _, err := runGit(projectPath, "check-ref-format", "--branch", name); err != nil {
		return nil, err
	}

	root, err := worktreesRoot()
	if err != nil {
		return nil, err
	}
	wtPath := filepath.Join(root, projectID, name)

	// Drop registrations whose directories are gone (a crash or manual rm), so
	// they don't block re-creating a worktree with the same name.
	if _, err := runGit(projectPath, "worktree", "prune"); err != nil {
		return nil, err
	}
	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("worktree path already exists: %s", wtPath)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return nil, fmt.Errorf("create worktrees dir: %w", err)
	}

	args := []string{"worktree", "add"}
	if baseIsRemote {
		remote, branch, ok := strings.Cut(base, "/")
		if !ok {
			return nil, fmt.Errorf("remote branch %q has no remote prefix", base)
		}
		if _, err := runGit(projectPath, "fetch", remote, branch); err != nil {
			return nil, err
		}
		args = append(args, "--track")
	}
	args = append(args, "-b", name, wtPath, base)
	if _, err := runGit(projectPath, args...); err != nil {
		return nil, err
	}

	// The session spawns claude at wtPath immediately after; make sure the
	// checkout actually works before handing it over.
	if _, err := runGit(wtPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("worktree created but unusable: %w", err)
	}
	return &Worktree{Name: name, Path: wtPath}, nil
}

// RemoveWorktree removes a worktree checkout. Without force git refuses to
// delete a dirty worktree, which is the safety net the close-session flow
// relies on; force discards uncommitted changes after the user has confirmed.
// The branch is never deleted either way.
func (s *Service) RemoveWorktree(projectPath, wtPath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)
	_, err := runGit(projectPath, args...)
	return err
}

// WorktreeDirty reports whether the worktree at wtPath has uncommitted changes
// (modified or untracked files) — the state that makes a plain remove fail.
func (s *Service) WorktreeDirty(wtPath string) (bool, error) {
	out, err := runGit(wtPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// branchExists reports whether refs/heads/<name> exists in the repository.
func branchExists(projectPath, name string) bool {
	err := exec.Command("git", "-C", projectPath, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run()
	return err == nil
}

// worktreesRoot resolves <XDG_DATA_HOME|~/.local/share>/lich/worktrees, the
// directory all app-created worktrees live under.
func worktreesRoot() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "lich", "worktrees"), nil
}

// splitLines splits command output into its non-empty lines.
func splitLines(out string) []string {
	lines := []string{}
	for line := range strings.Lines(out) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
