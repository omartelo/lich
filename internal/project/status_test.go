package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// v2 assembles a porcelain v2 -z payload the way git writes it: every record,
// headers included, terminated by a NUL rather than separated by one.
func v2(records ...string) string {
	var out strings.Builder
	for _, record := range records {
		out.WriteString(record)
		out.WriteString("\x00")
	}
	return out.String()
}

// TestParseWorkTreeBranchAndHead pins the two header lines against the
// contracts the separate git calls held. The sentinels are the whole point: v2
// spells a detached HEAD "(detached)" where Branch answers "", and a repository
// with no commits "(initial)" where the diff base is git's empty tree — mapped
// straight through, the badge would read a literal "(detached)".
func TestParseWorkTreeBranchAndHead(t *testing.T) {
	oid := "6d19bac07168c90c890aab654501e68498ac4d57"
	tests := []struct {
		name       string
		out        string
		wantBranch string
		wantHead   string
		wantBase   string
	}{
		{
			name:       "on a branch",
			out:        v2("# branch.oid "+oid, "# branch.head main"),
			wantBranch: "main",
			wantHead:   oid,
			wantBase:   "HEAD",
		},
		{
			name:       "detached HEAD names no branch",
			out:        v2("# branch.oid "+oid, "# branch.head (detached)"),
			wantBranch: "",
			wantHead:   oid,
			wantBase:   "HEAD",
		},
		{
			name:       "unborn repository has no head to diff against",
			out:        v2("# branch.oid (initial)", "# branch.head main"),
			wantBranch: "main",
			wantHead:   "",
			wantBase:   emptyTreeHash,
		},
		{
			name:       "upstream headers are ignored",
			out:        v2("# branch.oid "+oid, "# branch.head main", "# branch.upstream origin/main", "# branch.ab +0 -2"),
			wantBranch: "main",
			wantHead:   oid,
			wantBase:   "HEAD",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorkTree(tt.out)
			if got.branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", got.branch, tt.wantBranch)
			}
			if got.head != tt.wantHead {
				t.Errorf("head = %q, want %q", got.head, tt.wantHead)
			}
			if got.base != tt.wantBase {
				t.Errorf("base = %q, want %q", got.base, tt.wantBase)
			}
		})
	}
}

// TestParseWorkTreeCountsEveryDirtyEntryOnce covers the shapes v2 spells
// differently from the porcelain v1 line count it replaces. A rename is the one
// that bites: v1 kept "R orig -> new" on a single line, while v2 writes the
// original path as a second record of its own, so a naive split counts one
// renamed file twice.
func TestParseWorkTreeCountsEveryDirtyEntryOnce(t *testing.T) {
	oid := "6d19bac07168c90c890aab654501e68498ac4d57"
	blobs := "45b983be36b73c0788dc9cbcb76cbb80fc7bb057 45b983be36b73c0788dc9cbcb76cbb80fc7bb057"
	tests := []struct {
		name          string
		records       []string
		wantFiles     int
		wantUntracked []string
	}{
		{
			name:      "clean tree",
			records:   []string{"# branch.oid " + oid, "# branch.head main"},
			wantFiles: 0,
		},
		{
			name:      "modified and staged files",
			records:   []string{"1 .M N... 100644 100644 100644 " + blobs + " new.txt", "1 M. N... 100644 100644 100644 " + blobs + " staged.txt"},
			wantFiles: 2,
		},
		{
			name:      "a rename is one file, not two records",
			records:   []string{"2 R. N... 100644 100644 100644 " + blobs + " R100 renamed.txt", "old.txt"},
			wantFiles: 1,
		},
		{
			name:      "a copy spells its source the same way",
			records:   []string{"2 C. N... 100644 100644 100644 " + blobs + " C75 copy.txt", "source.txt", "? after.txt"},
			wantFiles: 2,
			// The source record must be consumed, or "source.txt" would be read
			// as the next entry and "? after.txt" lost behind it.
			wantUntracked: []string{"after.txt"},
		},
		{
			name:      "an unmerged path counts once",
			records:   []string{"u UU N... 100644 100644 100644 100644 " + blobs + " " + oid + " c.txt"},
			wantFiles: 1,
		},
		{
			name:          "untracked files are listed as well as counted",
			records:       []string{"? a.txt", "? pkg/deep/b.txt"},
			wantFiles:     2,
			wantUntracked: []string{"a.txt", "pkg/deep/b.txt"},
		},
		{
			name:      "ignored entries are neither counted nor listed",
			records:   []string{"! node_modules/x.js", "? a.txt"},
			wantFiles: 1,
			// The status call never asks for them; a git that volunteers one
			// must not inflate the badge.
			wantUntracked: []string{"a.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorkTree(v2(tt.records...))
			if got.files != tt.wantFiles {
				t.Errorf("files = %d, want %d", got.files, tt.wantFiles)
			}
			if strings.Join(got.untracked, "|") != strings.Join(tt.wantUntracked, "|") {
				t.Errorf("untracked = %q, want %q", got.untracked, tt.wantUntracked)
			}
		})
	}
}

// TestParseWorkTreeSurvivesMalformedOutput is the reason this parse is written
// the way it is: branch, head and dirty count now come out of one call, so a
// record it cannot read has to cost that record alone. Nothing here may panic,
// and the headers must still land.
func TestParseWorkTreeSurvivesMalformedOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{name: "empty output", out: ""},
		{name: "no trailing NUL", out: "# branch.head main"},
		{name: "header with no value", out: v2("# branch.head", "# branch.oid")},
		{name: "header with no key", out: v2("#", "# ")},
		{name: "untracked entry with no path", out: v2("?")},
		{name: "rename record with no source behind it", out: v2("# branch.head main", "2 R.")},
		{name: "a shape no git version has emitted", out: v2("$ who knows", "1", "u", "2")},
		{name: "nothing but separators", out: "\x00\x00\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorkTree(tt.out) // must not panic
			if got.base != emptyTreeHash && got.base != "HEAD" {
				t.Errorf("base = %q, want one of the two the contract allows", got.base)
			}
		})
	}
	if got := parseWorkTree(v2("# branch.head main", "2 R.")); got.branch != "main" {
		t.Errorf("branch = %q, want main — a bad entry must not cost the header", got.branch)
	}
}

// TestReadWorkTreeOnANonRepository proves the failure path hands back the same
// pair diffBase did, so DiffText's `git diff <base>` still fails as a git error
// rather than on some new shape.
func TestReadWorkTreeOnANonRepository(t *testing.T) {
	got := readWorkTree(t.TempDir())
	if got.branch != "" || got.head != "" || got.files != 0 || got.untracked != nil {
		t.Errorf("readWorkTree(non-repo) = %+v, want the zero readout", got)
	}
	if got.base != emptyTreeHash {
		t.Errorf("base = %q, want the empty tree", got.base)
	}
}

// TestReadWorkTreeAgainstRealGit ties the parse to the git actually installed:
// the table tests above pin shapes captured from git 2.55, and this is what
// notices when a git in the wild stops writing them.
func TestReadWorkTreeAgainstRealGit(t *testing.T) {
	repo, git := initRepo(t)

	clean := readWorkTree(repo)
	if clean.branch != "main" || clean.files != 0 || len(clean.head) < 7 {
		t.Errorf("readWorkTree(clean) = %+v, want main, no dirty entries, a head", clean)
	}

	// A staged rename, a modified tracked file and an untracked one in a fresh
	// directory: three dirty files, but four v2 records — the rename writes its
	// original path as a record of its own.
	if err := os.WriteFile(filepath.Join(repo, "kept.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "kept.txt")
	git("commit", "-m", "second file")
	git("mv", "a.txt", "b.txt")
	if err := os.WriteFile(filepath.Join(repo, "kept.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "pkg", "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := readWorkTree(repo)
	if dirty.files != 3 {
		t.Errorf("readWorkTree(renamed+modified+untracked).files = %d, want 3", dirty.files)
	}
	// git reports its paths with forward slashes on every platform.
	if len(dirty.untracked) != 1 || dirty.untracked[0] != "pkg/new.txt" {
		t.Errorf("untracked = %q, want [pkg/new.txt]", dirty.untracked)
	}

	git("checkout", "--detach", "HEAD")
	if got := readWorkTree(repo); got.branch != "" {
		t.Errorf("readWorkTree(detached).branch = %q, want empty", got.branch)
	}
}
