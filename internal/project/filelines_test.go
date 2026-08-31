package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// numbered builds a file whose every line names its own number, so a slice's
// content proves which lines came back and not merely how many.
func numbered(n int) string {
	var out strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&out, "line %d\n", i)
	}
	return out.String()
}

func TestSliceLines(t *testing.T) {
	text := "a\nb\nc\nd\n"
	tests := []struct {
		name     string
		text     string
		from, to int
		want     []string
	}{
		{"inner range", text, 2, 3, []string{"b", "c"}},
		{"single line", text, 1, 1, []string{"a"}},
		{"trailing newline opens no line", text, 4, 4, []string{"d"}},
		{"no trailing newline", "a\nb", 1, 2, []string{"a", "b"}},
		{"clamped to the last line", text, 3, 99, []string{"c", "d"}},
		{"past the end is empty, never nil", text, 9, 12, []string{}},
		{"blank lines are lines", "a\n\nc\n", 1, 3, []string{"a", "", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceLines(tt.text, tt.from, tt.to); !slices.Equal(got, tt.want) {
				t.Errorf("sliceLines(%q, %d, %d) = %q, want %q", tt.text, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// The cap is pinned to a literal and probed on both sides of it, so a change to
// maxExpandLines has to move this test rather than ride along with it.
func TestSliceLinesCap(t *testing.T) {
	if maxExpandLines != 500 {
		t.Fatalf("maxExpandLines = %d, want 500 — the wire contract moved", maxExpandLines)
	}
	text := numbered(1200)
	got := sliceLines(text, 1, 500)
	if len(got) != 500 || got[499] != "line 500" {
		t.Fatalf("500 lines: got %d ending %q", len(got), got[len(got)-1])
	}
	got = sliceLines(text, 1, 501)
	if len(got) != 500 || got[0] != "line 1" || got[499] != "line 500" {
		t.Fatalf("501 asked: got %d lines %q..%q", len(got), got[0], got[len(got)-1])
	}
	// The caller resumes where the cap stopped, which is what turns a gap
	// larger than the cap into several clicks instead of a truncated answer.
	got = sliceLines(text, 501, 1200)
	if len(got) != 500 || got[0] != "line 501" {
		t.Fatalf("resumed: got %d lines starting %q", len(got), got[0])
	}
}

func TestIsOID(t *testing.T) {
	valid := []string{"a4dbc1f", strings.Repeat("0", 40), "533f1af9fc740d2335cb3b4aae0e6481acfc5211"}
	for _, ref := range valid {
		if !isOID(ref) {
			t.Errorf("isOID(%q) = false, want true", ref)
		}
	}
	invalid := []string{"", "HEAD", "main", "a4dbc1", "--upload-pack=x", "A4DBC1F", "../etc", strings.Repeat("a", 65)}
	for _, ref := range invalid {
		if isOID(ref) {
			t.Errorf("isOID(%q) = true, want false", ref)
		}
	}
}

func TestContentsPath(t *testing.T) {
	got := contentsPath("internal/rpc/rpc go.md", "abcdef1234567890")
	want := "repos/{owner}/{repo}/contents/internal/rpc/rpc%20go.md?ref=abcdef1234567890"
	if got != want {
		t.Errorf("contentsPath = %q, want %q", got, want)
	}
}

func TestFileLinesWorkTree(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "src/app.ts", "one\ntwo\nthree\nfour\n")

	svc := New(nil)
	lines, err := svc.FileLines(repo, "src/app.ts", "", 2, 3)
	if err != nil {
		t.Fatalf("FileLines: %v", err)
	}
	if want := []string{"two", "three"}; !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}

	if _, err := svc.FileLines(repo, "../escape.ts", "", 1, 2); err == nil {
		t.Error("traversal: want error, got nil")
	}
	if _, err := svc.FileLines(repo, "src/app.ts", "", 0, 3); err == nil {
		t.Error("zero start: want error, got nil")
	}
	if _, err := svc.FileLines(repo, "src/app.ts", "", 4, 2); err == nil {
		t.Error("reversed range: want error, got nil")
	}
	write(t, repo, "blob.bin", "text\x00more\n")
	if _, err := svc.FileLines(repo, "blob.bin", "", 1, 2); err == nil {
		t.Error("binary: want error, got nil")
	}
}

// A local object is read by git, and never costs a gh call — the turn diff's
// snapshot tree is the case, and it is the reason the local read comes first.
func TestFileLinesLocalObject(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "a.txt", "one\ntwo\nthree\n")
	git("add", "a.txt")
	git("commit", "-m", "three lines")
	tree := strings.TrimSpace(git("rev-parse", "HEAD^{tree}"))
	// The working tree moves on; the object must not.
	write(t, repo, "a.txt", "rewritten\n")

	gh := &fakeGH{out: []byte("should not be asked\n")}
	svc := withGH(gh)
	lines, err := svc.FileLines(repo, "a.txt", tree, 2, 3)
	if err != nil {
		t.Fatalf("FileLines: %v", err)
	}
	if want := []string{"two", "three"}; !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
	if gh.calls != 0 {
		t.Errorf("gh called %d times for a local object, want 0", gh.calls)
	}
	if _, err := svc.FileLines(repo, "a.txt", "HEAD", 1, 2); err == nil {
		t.Error("non-oid revision: want error, got nil")
	}
}

func TestFileLinesLocalObjectRejectsADirectory(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "src/a.txt", "one\n")
	git("add", "src/a.txt")
	git("commit", "-m", "tree")
	tree := strings.TrimSpace(git("rev-parse", "HEAD^{tree}"))
	gh := &fakeGH{out: []byte("should not be asked\n")}

	if _, err := withGH(gh).FileLines(repo, "src", tree, 1, 2); err == nil {
		t.Fatal("directory at object: want error, got nil")
	}
	if gh.calls != 0 {
		t.Errorf("gh called %d times for a local tree, want 0", gh.calls)
	}
}

func TestFileLinesLocalObjectRejectsBinary(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "blob.bin", "text\x00more")
	git("add", "blob.bin")
	git("commit", "-m", "binary")
	tree := strings.TrimSpace(git("rev-parse", "HEAD^{tree}"))

	if _, err := New(nil).FileLines(repo, "blob.bin", tree, 1, 2); err == nil {
		t.Fatal("binary object: want error, got nil")
	}
}

// An oid the clone does not have is a pull request's head: GitHub answers for
// it, through the contents API rather than through git.
func TestFileLinesMissingObjectAsksGitHub(t *testing.T) {
	repo, _ := initRepo(t)
	missing := strings.Repeat("b", 40)

	gh := &fakeGH{out: []byte("alpha\nbeta\ngamma\n")}
	lines, err := withGH(gh).FileLines(repo, "a.txt", missing, 2, 9)
	if err != nil {
		t.Fatalf("FileLines: %v", err)
	}
	if want := []string{"beta", "gamma"}; !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
	want := []string{"api", "-H", ghRawAccept, contentsPath("a.txt", missing)}
	if !slices.Equal(gh.args, want) {
		t.Errorf("gh args = %v, want %v", gh.args, want)
	}
	if gh.dir != repo {
		t.Errorf("gh dir = %q, want %q", gh.dir, repo)
	}
}

func TestFileLinesGitHubRejectsOversize(t *testing.T) {
	repo, _ := initRepo(t)
	gh := &fakeGH{out: []byte(strings.Repeat("x", maxReadFileSize+1))}

	if _, err := withGH(gh).FileLines(repo, "a.txt", strings.Repeat("d", 40), 1, 2); err == nil {
		t.Fatal("oversize GitHub payload: want error, got nil")
	}
}

func TestFileLinesGitHubFailureSurfaces(t *testing.T) {
	repo, _ := initRepo(t)
	gh := &fakeGH{err: errors.New("gh: not found")}
	if _, err := withGH(gh).FileLines(repo, "a.txt", strings.Repeat("c", 40), 1, 5); err == nil {
		t.Error("gh failure: want error, got nil")
	}
}
