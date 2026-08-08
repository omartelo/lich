package terminal

import (
	"path/filepath"
	"testing"
)

// alwaysFree and neverFree stand in for the loopback probe, so the allocation
// tests below decide what the machine looks like instead of asking it.
func alwaysFree(int) bool { return true }
func neverFree(int) bool  { return false }

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

// TestPickWorktreePortPrefersHash proves an uncontested checkout still gets its
// hashed number, which is what keeps a worktree's port stable for free.
func TestPickWorktreePortPrefersHash(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	if got := pickWorktreePort(path, map[int]bool{}, alwaysFree); got != worktreePort(path) {
		t.Fatalf("pickWorktreePort(%q) = %d, want the hashed %d", path, got, worktreePort(path))
	}
}

// TestPickWorktreePortWalksPastReserved proves the collision this whole
// allocation exists for: a checkout whose hashed port another checkout already
// holds is moved off it rather than handed a number two dev servers would fight
// over.
func TestPickWorktreePortWalksPastReserved(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	hashed := worktreePort(path)
	got := pickWorktreePort(path, map[int]bool{hashed: true}, alwaysFree)
	if got == hashed {
		t.Fatalf("pickWorktreePort(%q) = %d, the port already reserved", path, got)
	}
	if got < worktreePortBase || got >= worktreePortBase+worktreePortCount {
		t.Fatalf("pickWorktreePort(%q) = %d, outside the window", path, got)
	}
}

// TestPickWorktreePortWalksPastBoundPort proves the half the reservation table
// cannot see: a port some unrelated process on this machine is already
// listening on is skipped too.
func TestPickWorktreePortWalksPastBoundPort(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	hashed := worktreePort(path)
	free := func(port int) bool { return port != hashed }
	if got := pickWorktreePort(path, map[int]bool{}, free); got == hashed {
		t.Fatalf("pickWorktreePort(%q) = %d, a port that failed to bind", path, got)
	}
}

// TestPickWorktreePortWrapsAtWindowEnd proves the walk is circular: with every
// port from the hashed one to the end of the window taken, the checkout is
// served from the start of the window rather than out of it.
func TestPickWorktreePortWrapsAtWindowEnd(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	hashed := worktreePort(path)
	inUse := map[int]bool{}
	for port := hashed; port < worktreePortBase+worktreePortCount; port++ {
		inUse[port] = true
	}
	got := pickWorktreePort(path, inUse, alwaysFree)
	if got != worktreePortBase {
		t.Fatalf("pickWorktreePort(%q) = %d, want the first port of the window %d",
			path, got, worktreePortBase)
	}
}

// TestPickWorktreePortExhaustedFallsBackToHash proves a full window costs the
// session nothing it did not already have: it gets its hashed number back,
// which is the collision the caller lived with before any of this.
func TestPickWorktreePortExhaustedFallsBackToHash(t *testing.T) {
	path := filepath.Join("src", "repo", "feature-a")
	if got := pickWorktreePort(path, map[int]bool{}, neverFree); got != worktreePort(path) {
		t.Fatalf("pickWorktreePort(%q) = %d, want the hashed %d", path, got, worktreePort(path))
	}
}

// TestPickWorktreePortWithoutPath proves a pathless session is not walked into
// a port: it has no checkout to own one.
func TestPickWorktreePortWithoutPath(t *testing.T) {
	if got := pickWorktreePort("", map[int]bool{}, alwaysFree); got != 0 {
		t.Fatalf("pickWorktreePort(\"\") = %d, want 0", got)
	}
}
