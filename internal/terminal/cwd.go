package terminal

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// cwdEventName carries a session's live working directory ({id, cwd, host}),
// emitted once with the directory the PTY starts in and again whenever the
// child process moves (the user runs `cd`). A non-empty host names a
// foreground process the directory cannot be read through (see shellHosts) and
// comes with an empty cwd: the session is somewhere, but not somewhere this
// machine can name. Global like the other session events: its consumer (the
// session card's path line) may be unmounted when it fires.
const cwdEventName = "session-cwd"

// cwdEvent is the payload of cwdEventName.
type cwdEvent struct {
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
	Host string `json:"host"`
}

// cwdPollInterval is how often a session's child working directory is read.
// Sub-second so a `cd` shows up promptly; each read is one cheap syscall
// (readlink on Linux, proc_pidinfo on macOS, a PEB walk on Windows) and emits
// only on change, so a static directory costs nothing to re-render.
const cwdPollInterval = 300 * time.Millisecond

// shellHosts are foreground commands that put the user's shell somewhere no
// local reader can follow: another machine (ssh, mosh), another namespace
// (docker, podman, nsenter, distrobox, toolbox, flatpak) or another terminal
// multiplexer whose panes are processes of their own (tmux, screen). Their own
// working directory is a real local path, which is exactly the danger — the
// readout would keep naming a directory the user is not in, and look right
// doing it.
//
// A named list rather than a test, because nothing a process exposes says "the
// shell you are watching is not mine": the pane, the container and the remote
// host are all invisible from here. A wrapper nobody listed still reads as an
// ordinary job.
var shellHosts = map[string]bool{
	"tmux":            true,
	"screen":          true,
	"ssh":             true,
	"mosh-client":     true,
	"docker":          true,
	"podman":          true,
	"nsenter":         true,
	"distrobox-enter": true,
	"toolbox":         true,
	"flatpak":         true,
}

// shellHost returns the name to publish when comm belongs to shellHosts, and
// "" for every ordinary foreground job. tmux writes its role into its own comm
// ("tmux: client"), so the match is on the command ahead of the colon; Linux
// truncates comm to 15 bytes, which every name above fits inside.
func shellHost(comm string) string {
	name := strings.TrimSpace(comm)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	if !shellHosts[name] {
		return ""
	}
	return name
}

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
	read := func() (string, string) { return processCwd(pid) }
	pollCwd(initial, ticker.C, done, read, func(cwd, host string) {
		hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd, Host: host})
	})
}

// pollCwd reads the session's location on every tick and calls emit whenever
// it differs from the last published one, returning when done closes. A read
// answering neither a directory nor a host (the process just died, done is
// about to close) is skipped rather than reported; a host arriving or clearing
// is a change even when the directory beside it does not move, because that is
// the moment the readout stops — or starts — being true. read is a parameter,
// like tick, so tests drive both without a process to point at.
func pollCwd(
	last string,
	tick <-chan time.Time,
	done <-chan struct{},
	read func() (string, string),
	emit func(cwd, host string),
) {
	lastHost := ""
	for {
		select {
		case <-done:
			return
		case <-tick:
			cwd, host := read()
			if cwd == "" && host == "" {
				continue
			}
			if cwd == last && host == lastHost {
				continue
			}
			last, lastHost = cwd, host
			emit(cwd, host)
		}
	}
}
