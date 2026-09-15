package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const revertBase = "alpha\nbravo\ncharlie\ndelta\necho\nfoxtrot\ngolf\nhotel\n"

// revertRepo commits code.txt with revertBase and then writes content over it,
// unstaged, so each case starts from one known diff.
func revertRepo(t *testing.T, content string) (string, func(args ...string) string) {
	t.Helper()
	repo, git := initRepo(t)
	writeFile(t, repo, "code.txt", revertBase)
	git("add", "code.txt")
	git("commit", "-m", "code")
	writeFile(t, repo, "code.txt", content)
	return repo, git
}

func writeFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, repo, rel string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(repo, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func TestRevertLinesAddition(t *testing.T) {
	repo, git := revertRepo(t, strings.Replace(revertBase, "bravo\n", "bravo\nNEW1\n", 1)+"TAIL\n")

	_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{{Side: "new", Line: 3, Text: "NEW1"}})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if got, want := readFile(t, repo, "code.txt"), revertBase+"TAIL\n"; got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
	if out := git("diff", "--cached"); out != "" {
		t.Errorf("index moved for an unstaged change: %q", out)
	}
}

func TestRevertLinesDeletion(t *testing.T) {
	repo, _ := revertRepo(t, strings.Replace(revertBase, "delta\n", "", 1))

	_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{{Side: "old", Line: 4, Text: "delta"}})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if got := readFile(t, repo, "code.txt"); got != revertBase {
		t.Errorf("content = %q, want HEAD", got)
	}
}

// TestRevertLinesPartOfABlock reverts the additions of a replace block and keeps
// its replacement line: the unpicked deletion must not come back.
func TestRevertLinesPartOfABlock(t *testing.T) {
	edited := strings.Replace(revertBase, "charlie\n", "CHARLIE\nextra1\nextra2\n", 1)
	repo, _ := revertRepo(t, edited)

	_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{
		{Side: "new", Line: 4, Text: "extra1"},
		{Side: "new", Line: 5, Text: "extra2"},
	})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	want := strings.Replace(revertBase, "charlie\n", "CHARLIE\n", 1)
	if got := readFile(t, repo, "code.txt"); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

// TestRevertLinesStagedChange reverts a fully staged change in the index too, so
// it cannot ride the next commit invisibly.
func TestRevertLinesStagedChange(t *testing.T) {
	repo, git := revertRepo(t, strings.Replace(revertBase, "echo\n", "ECHO\n", 1))
	git("add", "code.txt")

	result, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{
		{Side: "old", Line: 5, Text: "echo"},
		{Side: "new", Line: 5, Text: "ECHO"},
	})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if result.IndexPatch == "" {
		t.Error("index left alone for a staged change")
	}
	if out := git("status", "--porcelain"); out != "" {
		t.Errorf("status not clean: %q", out)
	}
}

// TestRevertLinesOtherHunkStaged keeps the index out when what is staged is a
// different change of the same file.
func TestRevertLinesOtherHunkStaged(t *testing.T) {
	repo, git := revertRepo(t, strings.Replace(revertBase, "alpha\n", "ALPHA\n", 1))
	git("add", "code.txt")
	writeFile(t, repo, "code.txt", strings.Replace(readFile(t, repo, "code.txt"), "hotel\n", "HOTEL\n", 1))

	result, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{
		{Side: "old", Line: 8, Text: "hotel"},
		{Side: "new", Line: 8, Text: "HOTEL"},
	})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if result.IndexPatch != "" {
		t.Error("index reverted, but the reverted lines were never staged")
	}
	if got := git("show", ":code.txt"); !strings.HasPrefix(got, "ALPHA\n") {
		t.Errorf("staged ALPHA lost: %q", got)
	}
}

func TestRevertLinesStagedDiverged(t *testing.T) {
	repo, git := revertRepo(t, strings.Replace(revertBase, "golf\n", "GOLF-staged\n", 1))
	git("add", "code.txt")
	edited := strings.Replace(revertBase, "golf\n", "GOLF-edited\n", 1)
	writeFile(t, repo, "code.txt", edited)

	_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{
		{Side: "old", Line: 7, Text: "golf"},
		{Side: "new", Line: 7, Text: "GOLF-edited"},
	})
	if !errors.Is(err, ErrStagedDiverged) {
		t.Fatalf("err = %v, want ErrStagedDiverged", err)
	}
	if got := readFile(t, repo, "code.txt"); got != edited {
		t.Errorf("file written on a refused revert: %q", got)
	}
}

func TestRevertLinesStale(t *testing.T) {
	repo, _ := revertRepo(t, strings.Replace(revertBase, "bravo\n", "bravo\nNEW1\n", 1))
	cases := map[string]RevertLine{
		"text moved":      {Side: "new", Line: 3, Text: "something else"},
		"line not change": {Side: "new", Line: 1, Text: "alpha"},
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{line})
			if !errors.Is(err, ErrLinesMoved) {
				t.Fatalf("err = %v, want ErrLinesMoved", err)
			}
		})
	}
}

func TestRevertLinesRejectsBadInput(t *testing.T) {
	repo, _ := revertRepo(t, revertBase+"TAIL\n")
	s := New(nil)
	if _, err := s.RevertLines(repo, "../escape.txt", []RevertLine{{Side: "new", Line: 9, Text: "TAIL"}}); err == nil {
		t.Error("escaping path accepted")
	}
	if _, err := s.RevertLines(repo, "code.txt", nil); err == nil {
		t.Error("empty selection accepted")
	}
	if _, err := s.RevertLines(repo, "code.txt", []RevertLine{{Side: "left", Line: 9, Text: "TAIL"}}); err == nil {
		t.Error("unknown side accepted")
	}
}

func TestRevertLinesUntracked(t *testing.T) {
	repo, git := initRepo(t)
	writeFile(t, repo, "new file.txt", "one\ntwo\nthree\n")

	_, err := New(nil).RevertLines(repo, "new file.txt", []RevertLine{{Side: "new", Line: 2, Text: "two"}})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if got := readFile(t, repo, "new file.txt"); got != "one\nthree\n" {
		t.Errorf("content = %q", got)
	}
	if out := git("ls-files", "new file.txt"); out != "" {
		t.Errorf("untracked file entered the index: %q", out)
	}
}

func TestRevertLinesNoNewlineAtEOF(t *testing.T) {
	repo, _ := revertRepo(t, strings.TrimSuffix(revertBase, "\n")+"\nindia")

	_, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{{Side: "new", Line: 9, Text: "india"}})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if got := readFile(t, repo, "code.txt"); got != revertBase {
		t.Errorf("content = %q, want HEAD", got)
	}
}

// TestRestoreLinesUndoesRevert round-trips both a working-tree-only and a staged
// revert back to where they started.
func TestRestoreLinesUndoesRevert(t *testing.T) {
	for _, staged := range []bool{false, true} {
		edited := strings.Replace(revertBase, "foxtrot\n", "FOXTROT\n", 1)
		repo, git := revertRepo(t, edited)
		if staged {
			git("add", "code.txt")
		}
		before := git("status", "--porcelain")
		s := New(nil)
		result, err := s.RevertLines(repo, "code.txt", []RevertLine{
			{Side: "old", Line: 6, Text: "foxtrot"},
			{Side: "new", Line: 6, Text: "FOXTROT"},
		})
		if err != nil {
			t.Fatalf("staged=%v RevertLines: %v", staged, err)
		}
		if err := s.RestoreLines(repo, "code.txt", result); err != nil {
			t.Fatalf("staged=%v RestoreLines: %v", staged, err)
		}
		if got := readFile(t, repo, "code.txt"); got != edited {
			t.Errorf("staged=%v content = %q", staged, got)
		}
		if after := git("status", "--porcelain"); after != before {
			t.Errorf("staged=%v status = %q, want %q", staged, after, before)
		}
	}
}

func TestRestoreLinesRefusesForeignPatch(t *testing.T) {
	repo, _ := revertRepo(t, revertBase)
	s := New(nil)
	foreign := RevertResult{Patch: patchFileHeader("a.txt") + "@@ -1 +1 @@\n-one\n+two\n"}
	if err := s.RestoreLines(repo, "code.txt", foreign); err == nil {
		t.Error("patch for another file accepted")
	}
	smuggled := RevertResult{Patch: patchFileHeader("code.txt") + "@@ -1 +1 @@\n-alpha\n+ALPHA\n--- a/a.txt\n+++ b/a.txt\n"}
	if err := s.RestoreLines(repo, "code.txt", smuggled); err == nil {
		t.Error("second file header accepted")
	}
	if err := s.RestoreLines(repo, "code.txt", RevertResult{Patch: patchFileHeader("code.txt") + "@@ -1 +1 @@\n-nope\n+NOPE\n"}); !errors.Is(err, ErrLinesMoved) {
		t.Errorf("err = %v, want ErrLinesMoved for a patch that no longer applies", err)
	}
}

// TestRevertLinesStagedWithUnstagedNeighbour reverts a staged change whose hunk
// also holds an unstaged one: both sides are reverted, and the neighbour stays.
func TestRevertLinesStagedWithUnstagedNeighbour(t *testing.T) {
	repo, git := revertRepo(t, strings.Replace(revertBase, "bravo\n", "BRAVO\n", 1))
	git("add", "code.txt")
	writeFile(t, repo, "code.txt", strings.Replace(readFile(t, repo, "code.txt"), "delta\n", "delta\nextra\n", 1))

	result, err := New(nil).RevertLines(repo, "code.txt", []RevertLine{
		{Side: "old", Line: 2, Text: "bravo"},
		{Side: "new", Line: 2, Text: "BRAVO"},
	})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if result.IndexPatch == "" {
		t.Error("index left alone for a staged change")
	}
	if got, want := readFile(t, repo, "code.txt"), strings.Replace(revertBase, "delta\n", "delta\nextra\n", 1); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
	if out := git("diff", "--cached"); out != "" {
		t.Errorf("staged change survived: %q", out)
	}
}

// TestRevertLinesStagedNewFile reverts a line of a file HEAD never had but the
// index does.
func TestRevertLinesStagedNewFile(t *testing.T) {
	repo, git := initRepo(t)
	writeFile(t, repo, "fresh.txt", "one\ntwo\n")
	git("add", "fresh.txt")

	result, err := New(nil).RevertLines(repo, "fresh.txt", []RevertLine{{Side: "new", Line: 2, Text: "two"}})
	if err != nil {
		t.Fatalf("RevertLines: %v", err)
	}
	if result.IndexPatch == "" {
		t.Error("index left alone for a staged new file")
	}
	if got := git("show", ":fresh.txt"); got != "one" { // gitIn trims the trailing newline
		t.Errorf("index content = %q", got)
	}
}
