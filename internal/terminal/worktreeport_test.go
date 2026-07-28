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

// TestWorktreePortStaysInRange proves every assigned port lands inside the
// documented window — below the ephemeral range and clear of lich's listener.
func TestWorktreePortStaysInRange(t *testing.T) {
	for i := range 5000 {
		path := filepath.Join("src", "repo", string(rune('a'+i%26)), string(rune(i)))
		port := worktreePort(path)
		if port < worktreePortBase || port >= worktreePortBase+worktreePortCount {
			t.Fatalf("worktreePort(%q) = %d, outside [%d, %d)",
				path, port, worktreePortBase, worktreePortBase+worktreePortCount)
		}
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
