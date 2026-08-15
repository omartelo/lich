package terminal

import (
	"bytes"
	"strconv"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// cwdEventName carries a session's live working directory ({id, cwd}), emitted
// once with the directory the PTY starts in and again whenever the child
// process moves (the user runs `cd`). Global like the other session events:
// its consumer (the session card's path line) may be unmounted when it fires.
const cwdEventName = "session-cwd"

// cwdEvent is the payload of cwdEventName.
type cwdEvent struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

// cwdPollInterval is how often a session's child working directory is read.
// Sub-second so a `cd` shows up promptly; each read is one cheap syscall
// (readlink on Linux, proc_pidinfo on macOS, a PEB walk on Windows) and emits
// only on change, so a static directory costs nothing to re-render.
const cwdPollInterval = 300 * time.Millisecond

// dosPathRunes is how many UTF-16 code units a DosPath of n bytes holds, and 0
// when n is not a length a UTF-16 buffer can have. n comes out of the child's
// own memory (the Windows processCwd walks its PEB), where x/sys documents it
// as "should always be even" — a 32-bit child, or one exiting mid-walk, hands
// back whatever happens to sit there. An odd n would size the buffer to zero
// and take the read path straight into a panic, on a bare goroutine that ends
// the process. Kept out of the build-tagged file so the guard tests on any OS.
func dosPathRunes(n uint16) int {
	if n == 0 || n%2 != 0 {
		return 0
	}
	return int(n / 2)
}

// parseTpgid returns the foreground process group of a /proc/<pid>/stat line —
// tpgid, the sixth field after the comm — and 0 when the line does not carry
// one. Counting starts at the last ')' because comm sits in parens and may hold
// spaces and parens of its own ("(sh (deploy))"), which is the whole reason a
// field split of the raw line is wrong. Kept out of the build-tagged file so
// the parse tests on any OS (dosPathRunes' pattern).
func parseTpgid(stat []byte) int {
	i := bytes.LastIndexByte(stat, ')')
	if i < 0 {
		return 0
	}
	fields := bytes.Fields(stat[i+1:])
	if len(fields) < 6 {
		return 0
	}
	pgrp, err := strconv.Atoi(string(fields[5]))
	if err != nil || pgrp < 0 {
		return 0
	}
	return pgrp
}

// watchCwd reports the session child's working directory to the frontend
// whenever it changes, until done closes. Each platform brings its own read
// (see the cwd_* files); on one without any, and when the PTY reports no
// child PID, this is a no-op.
func watchCwd(id string, pid int, initial string, done <-chan struct{}, hub *events.Hub) {
	if !cwdTracked || pid <= 0 {
		return
	}
	ticker := time.NewTicker(cwdPollInterval)
	defer ticker.Stop()
	pollCwd(pid, initial, ticker.C, done, func(cwd string) {
		hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd})
	})
}

// pollCwd reads pid's working directory on every tick and calls emit when it
// differs from the last seen value, returning when done closes. An unreadable
// directory (the process just died, done is about to close) is skipped rather
// than reported. Split from watchCwd so tests drive the ticks.
func pollCwd(pid int, last string, tick <-chan time.Time, done <-chan struct{}, emit func(string)) {
	for {
		select {
		case <-done:
			return
		case <-tick:
			cwd := processCwd(pid)
			if cwd == "" || cwd == last {
				continue
			}
			last = cwd
			emit(cwd)
		}
	}
}
