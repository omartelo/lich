//go:build linux || darwin

package terminal

import "golang.org/x/sys/unix"

// processCwd returns the working directory of whatever holds pid's terminal:
// pid itself while it is the foreground job — an agent, or a shell sitting at
// its prompt — and the foreground process group's leader once the user started
// one, which is the case the direct child cannot answer. A nested shell (`sh`,
// `su`, `nix develop`) does its own `cd`; the child that spawned it never
// moves, so its directory stops being the one the user means. Returns "" when
// neither can be read (the process exited, or was never ours to inspect).
//
// A tool subprocess is not a foreground job. Every coding agent spawns its own
// without a controlling terminal, so the agent's directory keeps answering
// through a `cd /tmp && …` the agent runs — which is why the foreground group
// is read and not the deepest descendant of the process tree.
//
// Each OS brings foregroundPgrp and readCwd (see the cwd_* files). Both are
// allowed to fail: an unreadable foreground group (a root shell under `sudo`,
// a leader that just exited) falls back to the direct child, so the readout is
// never worse than tracking the child alone.
func processCwd(pid int) string {
	if fg := foregroundPgrp(pid); fg > 0 && fg != pid && ownsTerminal(pid) {
		if cwd := readCwd(fg); cwd != "" {
			return cwd
		}
	}
	return readCwd(pid)
}

// ownsTerminal reports whether pid leads its own terminal session, which every
// PTY child does (the spawn calls setsid) and a process merely sharing someone
// else's terminal does not.
//
// Without this the read answers for the wrong terminal: a process started from
// a login shell reports that shell's foreground job, which is some sibling
// command in a directory that has nothing to do with it. lich only ever asks
// about a PTY child, but a function that is wrong when asked about anything
// else is a trap for whoever calls it next — and it is what makes this seam
// testable against the test binary itself, which shares the terminal `go test`
// was typed into.
func ownsTerminal(pid int) bool {
	sid, err := unix.Getsid(pid)
	return err == nil && sid == pid
}
