package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func matchKeys(matches []SearchMatch) string {
	keys := make([]string, len(matches))
	for i, m := range matches {
		keys[i] = fmt.Sprintf("%s:%d", m.Path, m.Line)
	}
	return strings.Join(keys, ",")
}

// TestSearchCoversWhatTheTreeLists proves the search reads the tree's own
// listing: tracked and untracked files hit, an ignored one never does, in the
// tree's order, with 1-based lines and the indentation dropped.
func TestSearchCoversWhatTheTreeLists(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "src/main.go", "package main\n\n\tfunc Needle() {}\n")
	write(t, repo, ".gitignore", "ignored.txt\n")
	git("add", ".")
	git("commit", "-m", "files")
	write(t, repo, "ignored.txt", "needle\n")
	write(t, repo, "untracked.txt", "a NEEDLE here\r\nnone\r\nneedle again\r\n")

	got, err := New(nil).Search(repo, "needle")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if keys, want := matchKeys(got.Matches), "src/main.go:3,untracked.txt:1,untracked.txt:3"; keys != want {
		t.Errorf("Search matches = %q, want %q", keys, want)
	}
	if text := got.Matches[0].Text; text != "func Needle() {}" {
		t.Errorf("Search text = %q, want the line without its indentation", text)
	}
	if text := got.Matches[1].Text; text != "a NEEDLE here" {
		t.Errorf("Search text = %q, want the line without its CR", text)
	}
	if got.Cut {
		t.Error("Search.Cut = true under the cap, want false")
	}
}

// TestSearchIsLiteral proves a query is text, not a pattern: regexp
// metacharacters match themselves.
func TestSearchIsLiteral(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "a.txt", "a.b\naxb\n")
	got, err := New(nil).Search(repo, "a.b")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if keys := matchKeys(got.Matches); keys != "a.txt:1" {
		t.Errorf("Search(a.b) = %q, want only the literal line", keys)
	}
}

// TestSearchWalksPlainFolder proves search works where the tree does, without git.
func TestSearchWalksPlainFolder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "notes.md", "find me\n")
	write(t, dir, "node_modules/dep.js", "find me\n")
	got, err := New(nil).Search(dir, "find")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if keys := matchKeys(got.Matches); keys != "notes.md:1" {
		t.Errorf("Search = %q, want only what the walk lists", keys)
	}
}

// TestSearchSkipsWhatThePreviewRefuses proves every hit is a file the preview
// opens: binaries, oversize files and symlinks never match.
func TestSearchSkipsWhatThePreviewRefuses(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "bin", "needle\x00")
	write(t, repo, "big", "needle"+strings.Repeat("x", maxReadFileSize))
	write(t, repo, "ok.txt", "needle\n")
	if err := os.Symlink(filepath.Join(repo, "ok.txt"), filepath.Join(repo, "link.txt")); err != nil {
		t.Logf("symlinks unavailable, skipping that case: %v", err)
	}
	got, err := New(nil).Search(repo, "needle")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if keys := matchKeys(got.Matches); keys != "ok.txt:1" {
		t.Errorf("Search = %q, want only ok.txt", keys)
	}
	if got.TooLarge != 1 {
		t.Errorf("Search.TooLarge = %d, want 1 (big only; a binary is not counted)", got.TooLarge)
	}
}

// TestSearchStopsAtTheCap proves a loose query stops at maxSearchMatches and
// says so.
func TestSearchStopsAtTheCap(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "many.txt", strings.Repeat("hit\n", maxSearchMatches+10))
	write(t, repo, "zz.txt", "hit\n")
	got, err := New(nil).Search(repo, "hit")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got.Matches) != maxSearchMatches || !got.Cut {
		t.Errorf("Search = %d matches, cut %v; want %d, cut true", len(got.Matches), got.Cut, maxSearchMatches)
	}
}

// TestSearchRefusesEmptyQuery proves a blank query is an error, not every line.
func TestSearchRefusesEmptyQuery(t *testing.T) {
	repo, _ := initRepo(t)
	if _, err := New(nil).Search(repo, "  "); err == nil {
		t.Error("Search(blank): want error, got nil")
	}
}

// TestSearchReportsMissingDir proves a checkout that is gone is an error, not
// "no matches".
func TestSearchReportsMissingDir(t *testing.T) {
	if _, err := New(nil).Search(filepath.Join(t.TempDir(), "gone"), "x"); err == nil {
		t.Error("Search(missing dir): want error, got nil")
	}
}

// TestExcerpt pins the cut of a long line: the match stays in view, a little of
// what precedes it survives, the cut never splits a rune, and each cut side is
// marked.
func TestExcerpt(t *testing.T) {
	short := "short line"
	if got := excerpt(short, 0); got != short {
		t.Errorf("excerpt(short) = %q, want it whole", got)
	}
	long := strings.Repeat("é", 200) + "NEEDLE" + strings.Repeat("é", 200)
	at := strings.Index(long, "NEEDLE")
	got := excerpt(long, at)
	if !strings.Contains(got, "NEEDLE") {
		t.Errorf("excerpt lost the match: %q", got)
	}
	if !strings.HasPrefix(got, "…é") || !strings.HasSuffix(got, "é…") {
		t.Errorf("excerpt = %q, want both sides marked and whole runes", got)
	}
	if body := strings.Trim(got, "…"); len(body) > 240 {
		t.Errorf("excerpt body is %d bytes, want at most 240", len(body))
	}
	head := excerpt("NEEDLE"+strings.Repeat("x", 300), 0)
	if strings.HasPrefix(head, "…") || !strings.HasSuffix(head, "…") {
		t.Errorf("excerpt at the start = %q, want only the tail marked", head)
	}
}
