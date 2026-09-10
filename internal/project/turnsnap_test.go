package project

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapIndex names a snapshot index under the test's own directory, absolute the
// way SnapshotTree requires.
func snapIndex(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "turn.idx")
}

// digest of a file, for proving one was left byte-identical.
func digest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return string(sum[:])
}

// The whole point of the pair: every kind of change a turn can make to the
// working tree reaches the rendered diff, whether or not the user staged it.
func TestSnapshotPairRendersEveryChange(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "sub/b.txt", "keep\n")
	git("add", "-A")
	git("commit", "-m", "second")
	index := snapIndex(t)

	before, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree (before): %v", err)
	}

	write(t, repo, "a.txt", "one\ntwo\n")
	write(t, repo, "c.txt", "new\n")
	if err := os.Remove(filepath.Join(repo, "sub", "b.txt")); err != nil {
		t.Fatal(err)
	}

	after, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree (after): %v", err)
	}
	if before == after {
		t.Fatal("a turn that edited, added and deleted files produced one tree oid")
	}

	diff, err := TreeDiff(repo, before, after)
	if err != nil {
		t.Fatalf("TreeDiff: %v", err)
	}
	for _, want := range []string{
		"diff --git a/a.txt b/a.txt", "+two",
		"+++ b/c.txt", "new file mode", "+new",
		"--- a/sub/b.txt", "deleted file mode", "-keep",
	} {
		if !strings.Contains(diff, want) {
			t.Errorf("diff missing %q:\n%s", want, diff)
		}
	}
}

// The one comparison the panel makes without running a diff: a turn that left
// the checkout as it found it has to be recognisable by the oids alone.
func TestSnapshotTreeIsStableWhenNothingChanged(t *testing.T) {
	repo, _ := initRepo(t)
	index := snapIndex(t)

	first, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	second, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if first != second {
		t.Fatalf("an unchanged worktree snapshotted as %s then %s", first, second)
	}
}

// A snapshot must be invisible to the user's own git: it runs while they are
// halfway through staging, and moving HEAD or their index would be data loss
// they never asked for.
func TestSnapshotTreeLeavesTheRepositoryAlone(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "staged.txt", "s\n")
	git("add", "staged.txt")
	write(t, repo, "loose.txt", "l\n")

	indexPath := filepath.Join(repo, ".git", "index")
	beforeIndex := digest(t, indexPath)
	beforeHead := git("rev-parse", "HEAD")
	beforeStatus := git("status", "--porcelain")
	beforeRefs := git("for-each-ref")

	if _, err := SnapshotTree(repo, snapIndex(t)); err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}

	if got := digest(t, indexPath); got != beforeIndex {
		t.Error("the snapshot rewrote .git/index")
	}
	if got := git("rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("HEAD moved: %q → %q", beforeHead, got)
	}
	if got := git("status", "--porcelain"); got != beforeStatus {
		t.Errorf("the working tree changed:\n%q\n%q", beforeStatus, got)
	}
	if got := git("for-each-ref"); got != beforeRefs {
		t.Error("the snapshot moved a ref")
	}
}

// Both diff sources have to agree about which files exist, or a file would be
// in one view and missing from the other with nothing saying why. DiffText's
// untracked pass obeys .gitignore; `add -A` does too, and this pins it.
func TestSnapshotTreeSkipsIgnoredFiles(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, ".gitignore", "build/\n")
	git("add", "-A")
	git("commit", "-m", "ignore")
	index := snapIndex(t)

	before, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	write(t, repo, "build/out.js", "generated\n")
	after, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if before != after {
		diff, _ := TreeDiff(repo, before, after)
		t.Fatalf("an ignored file changed the snapshot:\n%s", diff)
	}
}

// `-C dir` moves git before it resolves GIT_INDEX_FILE, so a relative path
// would name an index inside the user's checkout — a file lich writes into a
// repository it promised not to touch. Refused rather than resolved, because
// the caller that passed one is the bug.
func TestSnapshotTreeRefusesARelativeIndex(t *testing.T) {
	repo, _ := initRepo(t)
	if _, err := SnapshotTree(repo, "turn.idx"); err == nil {
		t.Fatal("a relative index path was accepted")
	}
}

// The index lives under lich's own directory, which need not exist yet on a
// fresh install — the first snapshot of the first session is what creates it.
func TestSnapshotTreeCreatesItsOwnDirectory(t *testing.T) {
	repo, _ := initRepo(t)
	index := filepath.Join(t.TempDir(), "turns", "s1.idx")

	if _, err := SnapshotTree(repo, index); err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("the index was not written: %v", err)
	}
}

// Browsing files is not a git feature, but bracketing a turn is: a plain folder
// has no trees to compare, and the panel has to hear that as an error rather
// than as a turn that changed nothing.
func TestSnapshotTreeOutsideARepository(t *testing.T) {
	if _, err := SnapshotTree(t.TempDir(), snapIndex(t)); err == nil {
		t.Fatal("SnapshotTree outside a repository: want an error, got nil")
	}
}

// diff-tree is plumbing and detects no rename on its own, where the porcelain
// `git diff` behind DiffText does. Without -M the two sources would describe the
// same change differently — a rename in one, a delete plus an add in the other.
func TestTreeDiffDetectsARename(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "long.txt", strings.Repeat("a stable line of content\n", 20))
	git("add", "-A")
	git("commit", "-m", "long")
	index := snapIndex(t)

	before, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if err := os.Rename(filepath.Join(repo, "long.txt"), filepath.Join(repo, "moved.txt")); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotTree(repo, index)
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}

	diff, err := TreeDiff(repo, before, after)
	if err != nil {
		t.Fatalf("TreeDiff: %v", err)
	}
	if !strings.Contains(diff, "rename from long.txt") || !strings.Contains(diff, "rename to moved.txt") {
		t.Errorf("the rename was not detected:\n%s", diff)
	}
}

// A snapshot whose index cannot be written is an error, never a silent success:
// the pair would otherwise look taken and the panel would report a turn that
// changed nothing when what actually happened is that nothing was recorded.
func TestSnapshotTreeReportsAnUnusableIndexLocation(t *testing.T) {
	repo, _ := initRepo(t)
	// A file where the index's directory should be, which is the shape a
	// misconfigured config directory takes on disk.
	blocker := filepath.Join(t.TempDir(), "turns")
	if err := os.WriteFile(blocker, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SnapshotTree(repo, filepath.Join(blocker, "s1.idx")); err == nil {
		t.Fatal("an unwritable index location was accepted")
	}
}
