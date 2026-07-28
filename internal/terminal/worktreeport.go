package terminal

import (
	"hash/fnv"
	"path/filepath"
)

// The window a checkout's dev-server port is drawn from. It sits below the
// ephemeral range Linux allocates outbound connections from (32768 by default),
// so an assigned port is never transiently held by an unrelated socket, and
// clear of both the ports frameworks default to (3000, 5173, 8080) and lich's
// own pinned listener (47821).
const (
	worktreePortBase  = 24000
	worktreePortCount = 1000
)

// worktreePort derives the dev-server port for the checkout at path: the same
// path always yields the same port, so a worktree keeps its number across a
// restart of lich and of the session, and two worktrees of one project stop
// fighting over the framework's default. Nothing is bound or probed here — the
// port is a name the checkout owns, and the dev server started in it is what
// actually claims it.
//
// An empty path has no checkout to name and yields 0, which the caller reads as
// "no port to export".
func worktreePort(path string) int {
	if path == "" {
		return 0
	}
	h := fnv.New32a()
	// Hash.Write never returns an error.
	_, _ = h.Write([]byte(filepath.Clean(path)))
	return worktreePortBase + int(h.Sum32()%worktreePortCount)
}
