package project

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BaseStatus is where a checkout stands against the branch it will merge into:
// how far that branch has moved underneath it, and which files a merge would
// collide on right now. It is what turns "the merge button refused" into
// something a parallel worktree knows before anyone clicks it.
type BaseStatus struct {
	Base      string   `json:"base"`   // the branch merged into, "origin/main" form
	Behind    int      `json:"behind"` // commits the base has that this checkout does not
	Conflicts []string `json:"conflicts"`
}

// baseFetchInterval throttles the background fetch behind the readout. Nothing
// else in lich fetches after a worktree is created, and a branch lands on the
// forge, not here — so without this the readout would answer from refs frozen at
// creation time and never notice the very thing it exists to report.
// The interval is a network round trip per repository with a card on screen,
// all day, so it is a cost as much as a cadence.
const baseFetchInterval = 30 * time.Second

// baseFetchTimeout caps the fetch subprocess. It already runs off the poll, but
// an ssh key prompt with nowhere to prompt would sit there for the app's life.
const baseFetchTimeout = 30 * time.Second

// BaseStatus reports the path's standing against its base branch, or nil when
// there is nothing to answer from: not a repository, no origin, no
// origin/HEAD. Nil is the "hide the segment" contract Branch and PullRequest
// share — this is polled, and a checkout without a remote is an ordinary state.
//
// The answer is memoised on the two commits it was computed from, because the
// poll runs every second and merge-tree writes tree objects into the object
// database on every call. Only a moved HEAD or a moved base recomputes.
func (s *Service) BaseStatus(path string) *BaseStatus {
	// One subprocess for all three, because this runs per second per checkout
	// and the poller it rides was already the frontend's git-subprocess budget.
	// --path-format=absolute: the common dir comes back relative from inside the
	// repository root, and two repositories would share the key ".git".
	out, ok := gitQuiet(path, "rev-parse", "--path-format=absolute",
		"--git-common-dir", "HEAD", "refs/remotes/origin/HEAD")
	if !ok {
		return nil
	}
	lines := splitLines(out)
	if len(lines) != 3 {
		return nil
	}
	repo, head, base := lines[0], lines[1], lines[2]

	// Fired on every call, throttled inside: a memo hit is exactly the case that
	// needs the refs to move for the answer to ever change.
	s.fetchBase(repo, path)

	if cached, ok := s.bases.answer(path, head, base); ok {
		return &cached
	}
	status := readBaseStatus(path, head, base)
	if status == nil {
		return nil
	}
	s.bases.remember(path, head, base, *status)
	return status
}

// readBaseStatus computes the readout from two resolved commits. Both are OIDs
// rather than ref names so a ref moving between the calls cannot mix two answers
// into one.
func readBaseStatus(path, head, base string) *BaseStatus {
	name, ok := gitQuiet(path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if !ok {
		return nil
	}
	behind, ok := commitsBehind(path, base, head)
	if !ok {
		return nil
	}
	status := BaseStatus{Base: strings.TrimSpace(name), Behind: behind}
	// A base that has not moved has nothing to merge in, so there is nothing to
	// collide with — and skipping merge-tree here is what keeps the common case
	// (every worktree, most of the day) down to one extra subprocess.
	if behind > 0 {
		status.Conflicts = mergeConflicts(path, base, head)
	}
	return &status
}

// commitsBehind counts the commits the base has that this checkout does not.
// `--left-right --count base...head` prints both sides, left first — the left
// side being the base alone, which is the one asked for here. The right side is
// read only to prove the line is the pair git promises rather than an error
// that happened to exit zero.
func commitsBehind(path, base, head string) (int, bool) {
	out, ok := gitQuiet(path, "rev-list", "--left-right", "--count", base+"..."+head)
	if !ok {
		return 0, false
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, false
	}
	behind, errLeft := strconv.Atoi(fields[0])
	_, errRight := strconv.Atoi(fields[1])
	if errLeft != nil || errRight != nil {
		return 0, false
	}
	return behind, true
}

// mergeConflicts lists the files a merge of base into head would conflict on,
// without touching the worktree, the index or HEAD: merge-tree computes the
// merge in the object database alone. Exit status 1 is its answer for "merged
// with conflicts" and the only status carrying a file list; anything else —
// including a git too old to know --write-tree — reports none, which degrades
// the readout to ahead/behind rather than failing it.
func mergeConflicts(dir, base, head string) []string {
	out, err := command("git", "-C", dir,
		"merge-tree", "--write-tree", "--name-only", "-z", base, head).Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return nil
	}
	return conflictPaths(string(out))
}

// conflictPaths reads merge-tree's -z output: the merged tree's OID, then the
// conflicted paths, then an empty entry closing that section off from the
// informational messages behind it.
func conflictPaths(out string) []string {
	entries := strings.Split(out, "\x00")
	if len(entries) < 2 {
		return nil
	}
	var paths []string
	for _, path := range entries[1:] {
		if path == "" {
			break
		}
		paths = append(paths, path)
	}
	return paths
}

// fetchBase updates the remote refs the readout is computed from, at most once
// per baseFetchInterval per repository and never on the caller's goroutine — the
// poll behind this answers a UI badge every second and cannot wait on a network
// round trip. A failed fetch is ordinary (offline, no credentials); it leaves
// the previous refs in place and the next interval tries again.
func (s *Service) fetchBase(repo, dir string) {
	if !s.bases.claimFetch(repo, time.Now()) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), baseFetchTimeout)
		defer cancel()
		cmd := commandContext(ctx, "git", "-C", dir, "fetch", "--quiet", "--no-tags", "origin")
		if err := cmd.Run(); err != nil {
			slog.Debug("base fetch", "dir", dir, "err", err)
		}
	}()
}

// baseCache holds what keeps the readout off the subprocess budget: the last
// answer per checkout, keyed on the commits it was computed from, and the next
// time each repository may be fetched. The zero value is ready to use.
type baseCache struct {
	mu        sync.Mutex
	answers   map[string]baseAnswer
	nextFetch map[string]time.Time
}

type baseAnswer struct {
	head   string
	base   string
	status BaseStatus
}

// answer returns the cached readout when it was computed from these very
// commits.
func (c *baseCache) answer(path, head, base string) (BaseStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.answers[path]
	if !ok || cached.head != head || cached.base != base {
		return BaseStatus{}, false
	}
	return cached.status, true
}

func (c *baseCache) remember(path, head, base string, status BaseStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.answers == nil {
		c.answers = make(map[string]baseAnswer)
	}
	c.answers[path] = baseAnswer{head: head, base: base, status: status}
}

// claimFetch reports whether the caller may fetch this repository now, and books
// the next slot as it says yes. Booking on the way out is also what keeps a slow
// fetch from being started a second time a second later: the claim is held for
// the whole interval, not for the length of the call.
func (c *baseCache) claimFetch(repo string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Before(c.nextFetch[repo]) {
		return false
	}
	if c.nextFetch == nil {
		c.nextFetch = make(map[string]time.Time)
	}
	c.nextFetch[repo] = now.Add(baseFetchInterval)
	return true
}
