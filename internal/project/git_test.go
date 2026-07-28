package project

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// captureLog points slog's default logger at a buffer for the duration of a
// test and returns what was written. runGit files a warning per failure, so the
// buffer is how a call is shown to be off that path.
func captureLog(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf.String
}

// initBareRepo makes a repository with no commits at all — the state where
// HEAD does not resolve, which every diff read has to treat as ordinary.
func initBareRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	return repo
}

// TestDiffOnARepositoryWithoutCommitsStaysSilent pins the contract that makes
// the footer poll affordable: Diff runs several git calls a second, per open
// checkout, and a repository with no commits fails the HEAD probe on every one
// of them. Routed through runGit that would file a warning per poll — thousands
// a day into a log that rotates at 5MB, burying the failures that mean
// something. The counts still have to be right, so both are asserted together.
func TestDiffOnARepositoryWithoutCommitsStaysSilent(t *testing.T) {
	repo := initBareRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logged := captureLog(t)

	stats := New(nil).Diff(repo)
	if stats.Files != 1 || stats.Added != 2 || stats.Deleted != 0 {
		t.Errorf("Diff = %+v, want 1 file and 2 added lines", stats)
	}
	if stats.Head != "" {
		t.Errorf("Head = %q, want empty on a repository with no commits", stats.Head)
	}
	if out := logged(); out != "" {
		t.Errorf("a polled Diff logged:\n%s", out)
	}
}

// TestBranchOnANonRepositoryStaysSilent covers the other polled read: the footer
// asks every open path for its branch, including ones that turned out not to be
// repositories at all.
func TestBranchOnANonRepositoryStaysSilent(t *testing.T) {
	logged := captureLog(t)

	if got := New(nil).Branch(t.TempDir()); got != "" {
		t.Errorf("Branch = %q, want empty outside a repository", got)
	}
	if out := logged(); out != "" {
		t.Errorf("a polled Branch logged:\n%s", out)
	}
}

// TestDiscardNewFileStaysSilent pins the probe whose negative answer is the
// result: "HEAD does not know this file" is what routes a discard to the
// delete-from-disk branch, so it is not a failure and must not read as one.
func TestDiscardNewFileStaysSilent(t *testing.T) {
	repo, _ := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "loose.txt"), []byte("l\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logged := captureLog(t)

	if err := New(nil).DiscardFile(repo, "loose.txt"); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}
	if out := logged(); out != "" {
		t.Errorf("discarding a new file logged:\n%s", out)
	}
}

// TestRunGitStillReportsRealFailures is the counterweight: the quiet runner
// exists for polls and probes, and must not have swallowed the failures a
// person is waiting on. A read of a directory that is not a repository still
// yields the screen's sentence and still reaches the log.
func TestRunGitStillReportsRealFailures(t *testing.T) {
	logged := captureLog(t)

	_, err := New(nil).Tree(t.TempDir())
	if err == nil {
		t.Fatal("Tree outside a repository: want an error, got nil")
	}
	if want := "not a git repository"; !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q does not name %q", err, want)
	}
	if !strings.Contains(logged(), "git failed") {
		t.Errorf("a reported failure never reached the log:\n%s", logged())
	}
}

// TestSplitNULDropsTheTrailingTerminator proves the shared splitter reads git's
// -z output the way git writes it: a NUL after every entry, so the last one is
// a terminator and never an empty path.
func TestSplitNULDropsTheTrailingTerminator(t *testing.T) {
	got := splitNUL("a.txt\x00dir/b.txt\x00")
	if len(got) != 2 || got[0] != "a.txt" || got[1] != "dir/b.txt" {
		t.Errorf("splitNUL = %q, want [a.txt dir/b.txt]", got)
	}
	if got := splitNUL(""); got != nil {
		t.Errorf("splitNUL(\"\") = %q, want nil", got)
	}
}
