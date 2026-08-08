package terminal

import (
	"hash/fnv"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
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

// worktreePort derives the port to offer the checkout at path first. Deriving
// it rather than counting from the base is what keeps two worktrees of one
// project apart on a fresh database, before either holds a reservation. An
// empty path has no checkout to name and yields 0, which the caller reads as
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

// pickWorktreePort chooses the port for the checkout at path: its hashed
// candidate when that one is neither reserved by another checkout (inUse) nor
// already bound on the machine (free), otherwise the next such port in the
// window, wrapping at its end. A hash over 1000 slots collides often enough to
// matter — roughly one project in six with twenty worktrees has a pair sharing
// a number — and it cannot see a port some unrelated process is sitting on at
// all. Either way both checkouts export a port only one of them can bind, and
// the one that loses says so as a dev server that will not start.
//
// With every port in the window spoken for it returns the hashed candidate:
// exhaustion is not a reason to leave a session without a port, and the
// collision it hands back is exactly what the caller had before.
//
// Nothing is bound or held here. The port is a name the checkout owns, and the
// dev server started in it is what actually claims it — a race lich cannot
// close, since between this call and that server starting the checkout's setup
// script has to run.
func pickWorktreePort(path string, inUse map[int]bool, free func(int) bool) int {
	first := worktreePort(path)
	if first == 0 {
		return 0
	}
	for offset := range worktreePortCount {
		port := worktreePortBase + (first-worktreePortBase+offset)%worktreePortCount
		if !inUse[port] && free(port) {
			return port
		}
	}
	return first
}

// reserveWorktreePort resolves the dev-server port for the checkout at cwd and
// makes sure the reservation table says so, so the next checkout to ask is told
// this number is spoken for. A checkout that already holds a reservation keeps
// it without a probe: the port a dev server is currently listening on must
// still be the one lich reports, and a bind test would read exactly that
// server as "taken".
//
// It runs under the caller's session lock, which is what serializes two
// sessions spawning at once. A failed write costs the reservation, not the
// session — the port is still exported, and the next spawn allocates again.
func (s *Service) reserveWorktreePort(cwd string) int {
	if cwd == "" {
		return 0
	}
	path := filepath.Clean(cwd)
	reserved := s.store.WorktreePorts()
	if port, ok := reserved[path]; ok {
		return port
	}
	inUse := make(map[int]bool, len(reserved))
	for _, port := range reserved {
		inUse[port] = true
	}
	port := pickWorktreePort(path, inUse, portFree)
	if err := s.store.SetWorktreePort(path, port); err != nil {
		slog.Warn("reserve worktree port", "path", path, "port", port, "err", err)
	}
	return port
}

// portFree reports whether port can be bound on the loopback interface. A
// listener already holding it on any address (0.0.0.0 included) makes the bind
// fail, which is the point: the hash knows nothing about what else runs on this
// machine.
func portFree(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
