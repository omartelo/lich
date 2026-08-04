package terminal

import (
	"path/filepath"
	"testing"
)

// TestWorktreePortIsStable pins the property the whole feature rests on: the
// port is a function of the path and nothing else, so a worktree keeps its
// number across restarts of the session and of lich.
func TestWorktreePortIsStable(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	first := worktreePort(path)
	for range 100 {
		if got := worktreePort(path); got != first {
			t.Fatalf("worktreePort(%q) = %d then %d, want a stable value", path, first, got)
		}
	}
}

// TestWorktreePortIgnoresPathNoise proves two spellings of one directory name
// the same port — otherwise a trailing slash would hand the same checkout two
// numbers depending on which caller asked.
func TestWorktreePortIgnoresPathNoise(t *testing.T) {
	clean := filepath.Join("src", "repo")
	noisy := filepath.Join("src", "nested", "..", "repo") + string(filepath.Separator)
	if worktreePort(clean) != worktreePort(noisy) {
		t.Fatalf("worktreePort(%q) = %d, worktreePort(%q) = %d, want equal",
			clean, worktreePort(clean), noisy, worktreePort(noisy))
	}
}

// TestWorktreePortSeparatesCheckouts proves the point of the feature: sibling
// worktrees of one project get different numbers, so both can run the dev
// server at once. Not a guarantee for arbitrary paths (the hash may collide),
// which is why these are named rather than generated.
func TestWorktreePortSeparatesCheckouts(t *testing.T) {
	seen := map[int]string{}
	for _, name := range []string{"repo", "repo-feature-a", "repo-feature-b", "repo-fix"} {
		path := filepath.Join("src", name)
		port := worktreePort(path)
		if other, dup := seen[port]; dup {
			t.Fatalf("%q and %q both got port %d", other, path, port)
		}
		seen[port] = path
	}
}

// lichListenPort duplicates defaultListenPort from package main, where it is a
// string and out of reach from here. Moving it there without moving it here is
// what the window test below is for.
const lichListenPort = 47821

// TestWorktreePortStaysInRange proves every assigned port lands inside the
// documented window. The bounds are literals on purpose: the window is a
// promise about the machine — below Linux's ephemeral range (32768), never
// lich's own listener — and comparing against the constants under test would
// only restate the arithmetic.
func TestWorktreePortStaysInRange(t *testing.T) {
	for i := range 5000 {
		path := filepath.Join("src", "repo", string(rune('a'+i%26)), string(rune(i)))
		port := worktreePort(path)
		if port < 24000 || port >= 32768 || port == lichListenPort {
			t.Fatalf("worktreePort(%q) = %d, outside the documented window", path, port)
		}
	}
}

// TestWorktreePortWindowClearsListener proves the whole window clears lich's
// pinned listener, not merely the paths the range test happens to sample: a
// checkout handed the app's own port would collide with lich itself.
func TestWorktreePortWindowClearsListener(t *testing.T) {
	if lichListenPort >= worktreePortBase && lichListenPort < worktreePortBase+worktreePortCount {
		t.Fatalf("window [%d, %d) contains lich's listener %d",
			worktreePortBase, worktreePortBase+worktreePortCount, lichListenPort)
	}
}

// TestWorktreePortWithoutPath proves an empty path names no checkout, so the
// caller is told there is no port to export rather than handed a number every
// pathless session would share.
func TestWorktreePortWithoutPath(t *testing.T) {
	if got := worktreePort(""); got != 0 {
		t.Fatalf("worktreePort(\"\") = %d, want 0", got)
	}
}
