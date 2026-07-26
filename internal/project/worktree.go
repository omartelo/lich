package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	cmd := command("git", append([]string{"-C", dir}, args...)...)
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

	list, err := runGit(path, "worktree", "list", "--porcelain")
	if err != nil {
		return branches, err
	}
	branches.Worktrees = append(branches.Worktrees, parseWorktrees(list)...)

	// A branch checked out in a linked worktree belongs only to the Worktrees
	// group, where selecting it resumes that worktree. Drop it from Local so the
	// same branch cannot also offer "create a new worktree off it" — the trap
	// that spawned a fresh worktree from a checkout the user meant to reopen.
	occupied := make(map[string]bool, len(branches.Worktrees))
	for _, wt := range branches.Worktrees {
		occupied[wt.Name] = true
	}

	local, err := runGit(path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return branches, err
	}
	for _, name := range splitLines(local) {
		if !occupied[name] {
			branches.Local = append(branches.Local, name)
		}
	}

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
		if first {
			first = false
			continue
		}
		if wt, ok := parseWorktreeBlock(block); ok {
			worktrees = append(worktrees, wt)
		}
	}
	return worktrees
}

// ListCheckouts returns every checkout of the repository holding a branch —
// the project's own directory included, which ListBranches deliberately leaves
// out because a main checkout is not something the worktree picker can resume.
// It answers a different question: which branches are spoken for. git refuses to
// check one branch out twice, so a caller about to check a branch out has to
// know about the main checkout as much as about the linked ones.
func (s *Service) ListCheckouts(path string) ([]Worktree, error) {
	out, err := runGit(path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseCheckouts(out), nil
}

// parseCheckouts reads every block of `git worktree list --porcelain`, main
// worktree included. Bare and detached entries still drop out: they hold no
// branch, so they can never be the reason a checkout is refused.
func parseCheckouts(out string) []Worktree {
	checkouts := []Worktree{}
	for block := range strings.SplitSeq(strings.TrimSpace(out), "\n\n") {
		if wt, ok := parseWorktreeBlock(block); ok {
			checkouts = append(checkouts, wt)
		}
	}
	return checkouts
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

// reserveWorktreePath resolves where the worktree holding branch name will live
// and leaves that path free and its parent created, so the caller has nothing
// left to do but hand it to git. Both creation flows share it.
//
// check-ref-format is git's own authority on a valid branch name, and it
// rejects "..", which is what keeps the Join below free of path traversal.
// Pruning first drops registrations whose directories are gone (a crash, a
// manual rm) so they cannot block re-creating a worktree under the same name,
// and an occupied path is refused here rather than by a half-finished git call.
func reserveWorktreePath(projectPath, projectID, name string) (string, error) {
	if _, err := runGit(projectPath, "check-ref-format", "--branch", name); err != nil {
		return "", err
	}
	root, err := worktreesRoot()
	if err != nil {
		return "", err
	}
	wtPath := filepath.Join(root, projectID, name)
	if _, err := runGit(projectPath, "worktree", "prune"); err != nil {
		return "", err
	}
	if _, err := os.Stat(wtPath); err == nil {
		return "", fmt.Errorf("worktree path already exists: %s", wtPath)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}
	return wtPath, nil
}

// CreateWorktree creates a git worktree named name (random when empty) under
// the app data dir, branching off base. A remote base is fetched first and the
// new branch tracks it. The worktree is verified usable before returning, so a
// success here means a session can be opened at Path right away.
func (s *Service) CreateWorktree(projectPath, projectID, name, base string, baseIsRemote bool) (*Worktree, error) {
	if name == "" {
		name = randomWorktreeName(func(n string) bool { return branchExists(projectPath, n) })
	}
	wtPath, err := reserveWorktreePath(projectPath, projectID, name)
	if err != nil {
		return nil, err
	}

	args := []string{"worktree", "add"}
	if baseIsRemote {
		remote, branch, ok := strings.Cut(base, "/")
		if !ok {
			return nil, fmt.Errorf("remote branch %q has no remote prefix", base)
		}
		if _, err := runGit(projectPath, "fetch", "--", remote, branch); err != nil {
			return nil, err
		}
		args = append(args, "--track")
	}
	// "--": base is unvalidated, and "-"-prefixed it would parse as a flag.
	args = append(args, "-b", name, "--", wtPath, base)
	if _, err := runGit(projectPath, args...); err != nil {
		return nil, err
	}

	// The session spawns claude at wtPath immediately after; make sure the
	// checkout actually works before handing it over.
	if _, err := runGit(wtPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, fmt.Errorf("worktree created but unusable: %w", err)
	}
	seedWorktree(projectPath, wtPath)
	return &Worktree{Name: name, Path: wtPath}, nil
}

// prHead is the little of a pull request CreateWorktreeFromPR needs: which
// branch to check out, and whether that branch lives on a fork.
type prHead struct {
	RefName   string `json:"headRefName"`
	CrossRepo bool   `json:"isCrossRepository"`
}

// pullRequestHead resolves a pull request's head branch through gh.
func (s *Service) pullRequestHead(path string, number int) (prHead, error) {
	out, err := s.gh(prReadTimeout, path, prArgs("view", number, "--json", "headRefName,isCrossRepository")...)
	if err != nil {
		return prHead{}, fmt.Errorf("gh pr view: %w", err)
	}
	var head prHead
	if err := json.Unmarshal(out, &head); err != nil {
		return prHead{}, fmt.Errorf("decode gh pr view: %w", err)
	}
	if head.RefName == "" {
		return prHead{}, fmt.Errorf("pull request #%d has no head branch", number)
	}
	return head, nil
}

// CreateWorktreeFromPR checks a pull request out into its own worktree under the
// app data dir, so a session can be opened on the PR's own branch. The checkout
// is created detached and handed to `gh pr checkout`, which owns resolving the
// head ref and its tracking — the part raw git cannot do for a pull request.
//
// It refuses a PR from a fork: the branch would check out, but the commits an
// agent makes there have nowhere to push, so the failure belongs before the
// worktree exists rather than at the first push. A failed checkout takes the
// worktree with it — a detached husk would sit in the picker offering a resume
// of nothing, and would hold the path against a second attempt.
func (s *Service) CreateWorktreeFromPR(projectPath, projectID string, number int) (*Worktree, error) {
	if number <= 0 {
		return nil, fmt.Errorf("invalid pull request number %d", number)
	}
	head, err := s.pullRequestHead(projectPath, number)
	if err != nil {
		return nil, err
	}
	if head.CrossRepo {
		return nil, fmt.Errorf("pull request #%d comes from a fork: its branch cannot be pushed back", number)
	}
	wtPath, err := reserveWorktreePath(projectPath, projectID, head.RefName)
	if err != nil {
		return nil, err
	}
	if _, err := runGit(projectPath, "worktree", "add", "--detach", wtPath); err != nil {
		return nil, err
	}
	if _, err := s.gh(prCheckoutTimeout, wtPath, "pr", "checkout", strconv.Itoa(number)); err != nil {
		if _, rmErr := runGit(projectPath, "worktree", "remove", "--force", wtPath); rmErr != nil {
			return nil, fmt.Errorf("gh pr checkout: %w (and the worktree could not be removed: %v)", err, rmErr)
		}
		return nil, fmt.Errorf("gh pr checkout: %w", err)
	}
	seedWorktree(projectPath, wtPath)
	return &Worktree{Name: head.RefName, Path: wtPath}, nil
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
	err := command("git", "-C", projectPath, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run()
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
