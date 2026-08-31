package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	checkouts := parseCheckouts(out)
	// The caller's own directory is handed back spelled the way the caller
	// named it. A project's path is its identity in the workspace — it is what
	// projectID hashes and what every stored session carries — so a checkout
	// the caller cannot recognise as "the project itself" sends it down the
	// worktree flows, which is precisely what this call exists to prevent.
	own := canonicalPath(path)
	for i, c := range checkouts {
		if c.Path == own {
			checkouts[i].Path = filepath.Clean(path)
		}
	}
	return checkouts, nil
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

// canonicalPath renders a checkout path the one way lich compares them. git
// reports a worktree by its fully resolved path, while lich builds one by
// joining names onto the data dir — the same directory under two spellings the
// moment a symlink is in the way (macOS puts every temp and /var path behind
// one) or the platform has more than one form for a name (Windows 8.3). Every
// path that will be compared goes through here, on both sides.
//
// A path that no longer exists (a stale registration) cannot be resolved; Clean
// is the best available answer and keeps the entry readable.
func canonicalPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}

// parseWorktreeBlock extracts one worktree entry; ok is false for bare or
// detached entries and malformed blocks.
func parseWorktreeBlock(block string) (Worktree, bool) {
	var wt Worktree
	for line := range strings.SplitSeq(block, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if p := strings.TrimPrefix(line, "worktree "); p != "" {
				wt.Path = canonicalPath(p)
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
	// check-ref-format says no by exit status alone, with nothing on stderr, so
	// the name it rejected has to come from here.
	if _, err := runGit(projectPath, "check-ref-format", "--branch", name); err != nil {
		return "", fmt.Errorf("%q is not a name git accepts for a branch.", name)
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
		return "", fmt.Errorf("A worktree named %q already exists.", name)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		slog.Warn("create worktrees dir", "path", filepath.Dir(wtPath), "err", err)
		return "", errors.New("The worktree directory could not be created.")
	}
	return wtPath, nil
}

// baseFetchBudget caps the fetch that precedes a worktree on a remote base.
// It is the one part of creating a worktree that reaches the network, and the
// dialog waits on it: unbudgeted, a remote that wants a passphrase leaves the
// create call hanging for the app's life.
const baseFetchBudget = 60 * time.Second

// fetchBase updates refs/remotes/<remote>/<branch> and reports whether the ref
// is there afterwards, which is the question `worktree add` is about to ask.
//
// git's exit status alone does not answer it: under a narrowed refspec — a
// --single-branch clone, `remote add -t`, a hand-edited remote.<name>.fetch —
// `git fetch origin -- other` writes the branch to FETCH_HEAD, says "* branch
// other -> FETCH_HEAD", and exits 0 without creating the remote-tracking ref.
// So a fetch that succeeded is believed only once the ref is there.
//
// A fetch that *failed* is a refusal even when the ref is there, because then it
// is whatever an earlier fetch left behind: a worktree silently based on a
// week-old ref is worse than one that was never created. The exception is a ref
// that moved during this call — a concurrent fetch landed what this one needed,
// which is what two sessions creating a worktree at once do to each other. One
// that found nothing new leaves the ref where it was and is refused with it;
// retrying the create is the whole cost.
func fetchBase(projectPath, remote, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), baseFetchBudget)
	defer cancel()

	ref := "refs/remotes/" + remote + "/" + branch
	before, _ := gitQuiet(projectPath, "rev-parse", "--verify", "--quiet", ref)

	args := []string{"-C", projectPath, "fetch", "--", remote, branch}
	cmd := commandContext(ctx, "git", args...)
	// Without these a remote that wants a credential or a key passphrase stops
	// on a prompt with no terminal to answer it — or worse, a GUI askpass dialog
	// behind the app window — and burns the whole budget waiting for a person.
	// Appended to what prepare set, never assigned over it (git.go).
	cmd.Env = append(cmd.Env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	fetchErr := cmd.Run()

	after, ok := gitQuiet(projectPath, "rev-parse", "--verify", "--quiet", ref)
	if ok && (fetchErr == nil || after != before) {
		return nil
	}
	if fetchErr != nil {
		logToolFailure("git", args, stderr.String(), fetchErr)
		if ctx.Err() != nil {
			return fmt.Errorf("Fetching %s took longer than %s and was given up on.", ref, baseFetchBudget)
		}
		return errors.New(gitMessage(stderr.String(), fetchErr))
	}
	return fmt.Errorf("git fetched %s but this repository does not keep %s — its remote only tracks some branches.", branch, ref)
}

// CreateWorktree creates a git worktree named name (random when empty) under
// the app data dir, branching off base. A remote base is fetched first, and the
// new branch is left with no upstream. A branch that already exists is checked
// out as it stands, and base is ignored. The worktree is verified usable before
// returning, so a success here means a session can be opened at Path right away.
func (s *Service) CreateWorktree(projectPath, projectID, name, base string, baseIsRemote bool) (*Worktree, error) {
	if name == "" {
		name = randomWorktreeName(func(n string) bool { return branchExists(projectPath, n) })
	}
	wtPath, err := reserveWorktreePath(projectPath, projectID, name)
	if err != nil {
		return nil, err
	}

	// A branch that already exists is checked out as it stands, never recreated:
	// naming a branch is naming the work on it, the same reading
	// spawn.resolveCheckout gives a branch that already has a checkout. git's
	// -b refuses a name it already knows, so the only way to open a session on a
	// branch prepared in advance used to be deleting it first. base drops out
	// with it — it decides where a branch starts, and this one already started.
	args := []string{"worktree", "add"}
	source := base
	if branchExists(projectPath, name) {
		source = name
	} else {
		if baseIsRemote {
			remote, branch, ok := strings.Cut(base, "/")
			if !ok {
				return nil, fmt.Errorf("%q is not a remote branch lich can track.", base)
			}
			if err := fetchBase(projectPath, remote, branch); err != nil {
				return nil, err
			}
			// The full refname, never the "origin/main" shorthand: a repository
			// that also holds a *local* branch literally named "origin/main" has
			// two refs answering to that shorthand, and git refuses the pair
			// outright ("ambiguous object name") — a refusal the person reading
			// it cannot connect to a branch name they may not know exists.
			source = "refs/remotes/" + base
			// --no-track, because autoSetupMerge would otherwise make the base
			// this branch's upstream: a `git pull` in the session's terminal
			// would merge the base into the work, and `git status` would report
			// the work as "ahead of origin/main". Which upstream a branch gets
			// is the first push's question, and the user's to answer.
			args = append(args, "--no-track")
		}
		args = append(args, "-b", name)
	}
	// "--": source is unvalidated, and "-"-prefixed it would parse as a flag.
	args = append(args, "--", wtPath, source)
	if _, err := runGit(projectPath, args...); err != nil {
		return nil, err
	}

	// The session spawns its provider at wtPath immediately after; make sure
	// the checkout actually works before handing it over.
	if _, err := runGit(wtPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, errors.New("The worktree was created but git does not read it as a checkout.")
	}
	seedWorktree(projectPath, wtPath)
	return &Worktree{Name: name, Path: canonicalPath(wtPath)}, nil
}

// prHead is the little of a pull request CreateWorktreeFromPR needs: which
// branch to check out, whether that branch lives on a fork, and whether the fork
// takes pushes from a maintainer.
type prHead struct {
	RefName   string `json:"headRefName"`
	CrossRepo bool   `json:"isCrossRepository"`
	CanModify bool   `json:"maintainerCanModify"`
}

// pullRequestHead resolves a pull request's head branch through gh.
func (s *Service) pullRequestHead(path string, number int) (prHead, error) {
	out, err := s.gh(prReadTimeout, path, prArgs("view", number, "--json", "headRefName,isCrossRepository,maintainerCanModify")...)
	if err != nil {
		return prHead{}, err
	}
	var head prHead
	if err := json.Unmarshal(out, &head); err != nil {
		return prHead{}, errUnreadableAnswer(err)
	}
	if head.RefName == "" {
		return prHead{}, fmt.Errorf("GitHub reports no head branch for pull request #%d.", number)
	}
	return head, nil
}

// CreateWorktreeFromPR checks a pull request out into its own worktree under the
// app data dir, so a session can be opened on the PR's own branch. The checkout
// is created detached and handed to `gh pr checkout`, which owns resolving the
// head ref and its tracking — the part raw git cannot do for a pull request.
//
// It refuses a PR from a fork that has "allow edits by maintainers" off: the
// branch would check out, but the commits an agent makes there have nowhere to
// push, so the failure belongs before the worktree exists rather than at the
// first push. With the permission on, the fork's branch takes a push like any
// other and the pull request opens. A failed checkout takes the worktree with it
// — a detached husk would sit in the picker offering a resume of nothing, and
// would hold the path against a second attempt.
func (s *Service) CreateWorktreeFromPR(projectPath, projectID string, number int) (*Worktree, error) {
	if number <= 0 {
		return nil, fmt.Errorf("%d is not a pull request number.", number)
	}
	head, err := s.pullRequestHead(projectPath, number)
	if err != nil {
		return nil, err
	}
	if head.CrossRepo && !head.CanModify {
		return nil, fmt.Errorf("Pull request #%d comes from a fork that does not allow edits by maintainers: its branch could not be pushed back.", number)
	}
	wtPath, err := reserveWorktreePath(projectPath, projectID, head.RefName)
	if err != nil {
		return nil, err
	}
	if _, err := runGit(projectPath, "worktree", "add", "--detach", wtPath); err != nil {
		return nil, err
	}
	// The checkout runs inside the new worktree but answers to the project's
	// account: the worktree has no session row yet, so it can name no account
	// of its own.
	if _, err := s.ghFor(projectPath, prCheckoutTimeout, wtPath, "pr", "checkout", strconv.Itoa(number)); err != nil {
		if _, rmErr := runGit(projectPath, "worktree", "remove", "--force", wtPath); rmErr != nil {
			slog.Warn("worktree left behind by a failed PR checkout", "path", wtPath, "err", rmErr)
			return nil, fmt.Errorf("%w Its half-made worktree could not be removed either.", err)
		}
		return nil, err
	}
	seedWorktree(projectPath, wtPath)
	return &Worktree{Name: head.RefName, Path: canonicalPath(wtPath)}, nil
}

// WorktreeAdopted reports whether the checkout at wtPath is one lich adopted
// rather than created: `git worktree list` hands back every worktree of a
// repository, so one the user made by hand appears in the picker and hosts a
// session like any other, and nothing but its path tells the two apart.
// Everything lich creates lives under the worktrees root reserveWorktreePath
// builds its paths in; anything outside it is the user's own directory, which
// lich may forget but never delete.
//
// Both sides go through canonicalPath, and an unresolvable root reads as
// adopted: the answer gates a deletion, so the unknown case has to be the one
// that keeps the checkout.
func (s *Service) WorktreeAdopted(wtPath string) bool {
	root, err := worktreesRoot()
	if err != nil {
		return true
	}
	// Rel, not a string prefix: "<root>-old" starts with the root spelled out
	// and is no more lich's than any other directory next to it.
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(wtPath))
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// RemoveWorktree removes a worktree checkout. Without force git refuses to
// delete a dirty worktree, which is the safety net the close-session flow
// relies on; force discards uncommitted changes after the user has confirmed.
// The branch is never deleted either way.
//
// An adopted checkout is refused outright, force or not: the directory is the
// user's, made outside lich and only listed by it, and --force would take
// uncommitted work with it. Callers ask WorktreeAdopted before they take a
// session apart, so this is the invariant behind them rather than the message
// anyone reads — but it is the one that holds when a caller forgets.
func (s *Service) RemoveWorktree(projectPath, wtPath string, force bool) error {
	if s.WorktreeAdopted(wtPath) {
		return fmt.Errorf("The worktree at %s was not created by lich, so lich will not delete it.", wtPath)
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	// "--": same guard CreateWorktree uses — the path is unvalidated here too.
	args = append(args, "--", wtPath)
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

// branchExists reports whether refs/heads/<name> exists in the repository. A
// name that does not exist is the answer the caller is looking for, not a
// failure, so the probe stays off runGit's reporting path.
func branchExists(projectPath, name string) bool {
	_, ok := gitQuiet(projectPath, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return ok
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
