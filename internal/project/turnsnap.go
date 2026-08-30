package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SnapshotTree records a checkout's whole working tree — tracked edits,
// deletions and untracked files alike — as a git tree object, and returns its
// oid. Two of these bracket a session's turn, and the pair is what TreeDiff
// renders (see internal/terminal/turnsnap.go).
//
// It writes nothing but loose objects. index is an index file of lich's own, so
// `.git/index`, HEAD, the refs and the working tree are all untouched: a
// snapshot taken while the user is halfway through their own `git add` neither
// sees nor disturbs it. It must be ABSOLUTE, because `-C dir` moves git before
// it resolves the variable, and a relative path would silently name an index
// inside the checkout.
//
// The same file has to be reused across a session's snapshots. git's stat cache
// is what makes the second one cheap — measured on a 37k-file checkout, 2.9s
// against an empty index and 170ms against a warm one — so a per-call temporary
// would re-hash the whole worktree every turn.
//
// Ignored files are absent: `add -A` obeys .gitignore, which is the same rule
// DiffText's untracked pass follows, so the two diff sources never disagree
// about which files exist.
func SnapshotTree(dir, index string) (string, error) {
	if !filepath.IsAbs(index) {
		return "", fmt.Errorf("snapshot index %q must be absolute", index)
	}
	if err := os.MkdirAll(filepath.Dir(index), 0o700); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}
	if _, err := snapGit(dir, index, "add", "-A"); err != nil {
		return "", err
	}
	out, err := snapGit(dir, index, "write-tree")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(out)
	if oid == "" {
		return "", fmt.Errorf("git write-tree in %s said nothing", dir)
	}
	return oid, nil
}

// TreeDiff renders the change between two snapshots as a unified diff — the
// same text `git diff` produces, which is what parseDiff and FileDiff already
// consume. -M is what keeps it the same text: diff-tree is plumbing and detects
// no rename without it, where the porcelain `git diff` behind DiffText does.
func TreeDiff(dir, before, after string) (string, error) {
	return runGit(dir, "diff-tree", "-p", "-M", before, after)
}

// snapGit runs one snapshot call against an index of lich's own. It reports
// failures like runGit rather than swallowing them like gitQuiet: a snapshot
// that fails costs the panel a whole turn, and the reason has to reach the log.
func snapGit(dir, index string, args ...string) (string, error) {
	cmd := command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Env, "GIT_INDEX_FILE="+index)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", gitFailure(args, stderr.String(), err)
	}
	return string(out), nil
}
