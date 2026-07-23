package project

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestMatchesInclude proves the reduced gitignore semantics: slashless
// patterns match the basename at any depth, slashed patterns match the whole
// repo-relative path, and an invalid pattern matches nothing instead of
// panicking.
func TestMatchesInclude(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		patterns []string
		want     bool
	}{
		{"basename at root", ".env", []string{".env*"}, true},
		{"basename nested", "api/.env.local", []string{".env*"}, true},
		{"no match", "src/main.go", []string{".env*"}, false},
		{"path pattern hits", "config/local.json", []string{"config/*.json"}, true},
		{"path pattern misses sibling", "other/local.json", []string{"config/*.json"}, false},
		{"path pattern is not recursive", "config/sub/local.json", []string{"config/*.json"}, false},
		{"invalid pattern ignored", ".env", []string{"["}, false},
		{"second pattern wins", "id_rsa", []string{".env*", "id_*"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesInclude(tt.rel, tt.patterns); got != tt.want {
				t.Errorf("matchesInclude(%q, %v) = %v, want %v", tt.rel, tt.patterns, got, tt.want)
			}
		})
	}
}

// TestIncludePatterns proves the override semantics: no file falls back to the
// default, a present file governs alone (comments and blanks skipped), and an
// empty file means seed nothing.
func TestIncludePatterns(t *testing.T) {
	dir := t.TempDir()
	if got := includePatterns(dir); !slices.Equal(got, defaultIncludes) {
		t.Errorf("no file: patterns = %v, want %v", got, defaultIncludes)
	}

	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, includeFile), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("# secrets\n\nconfig/*.json\n.npmrc\n")
	if got := includePatterns(dir); !slices.Equal(got, []string{"config/*.json", ".npmrc"}) {
		t.Errorf("file: patterns = %v, want [config/*.json .npmrc]", got)
	}

	write("# only comments\n\n")
	if got := includePatterns(dir); len(got) != 0 {
		t.Errorf("empty file: patterns = %v, want none", got)
	}
}

// seedRepo builds a repository with a .gitignore and the ignored files the
// seed should (and should not) pick up.
func seedRepo(t *testing.T) (string, func(args ...string) string) {
	t.Helper()
	repo, git := initRepo(t)
	files := map[string]string{
		".gitignore":                ".env*\nnode_modules/\n",
		".env":                      "SECRET=1\n",
		"api/.env.local":            "LOCAL=1\n",
		"node_modules/pkg/index.js": "js\n",
		"notes.txt":                 "untracked but not ignored\n",
	}
	for rel, content := range files {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Join(repo, ".env"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".gitignore")
	git("commit", "-m", "ignore")
	return repo, git
}

// TestCreateWorktreeSeedsIgnored proves a new worktree receives the ignored
// env files — nested ones included, permissions preserved — while ignored
// directories and plain untracked files stay behind.
func TestCreateWorktreeSeedsIgnored(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, _ := seedRepo(t)

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "seeded", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	env, err := os.ReadFile(filepath.Join(wt.Path, ".env"))
	if err != nil {
		t.Fatalf(".env not seeded: %v", err)
	}
	if string(env) != "SECRET=1\n" {
		t.Errorf(".env content = %q, want SECRET=1", env)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(wt.Path, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf(".env perm = %o, want 600", perm)
		}
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "api", ".env.local")); err != nil {
		t.Errorf("nested .env.local not seeded: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("node_modules seeded, want absent (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("plain untracked notes.txt seeded, want absent (err=%v)", err)
	}
}

// TestCreateWorktreeSeedsFromIncludeFile proves .worktreeinclude replaces the
// default patterns instead of extending them.
func TestCreateWorktreeSeedsFromIncludeFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo, git := seedRepo(t)

	if err := os.MkdirAll(filepath.Join(repo, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config", "local.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env*\nnode_modules/\nconfig/local.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".worktreeinclude"), []byte("config/*.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".gitignore", ".worktreeinclude")
	git("commit", "-m", "include file")

	svc := New(nil)
	wt, err := svc.CreateWorktree(repo, "pid", "included", "main", false)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "config", "local.json")); err != nil {
		t.Errorf("config/local.json not seeded: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, ".env")); !os.IsNotExist(err) {
		t.Errorf(".env seeded despite .worktreeinclude override (err=%v)", err)
	}
}
