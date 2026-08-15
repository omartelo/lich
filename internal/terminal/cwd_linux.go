//go:build linux

package terminal

import (
	"os"
	"strconv"
)

// cwdTracked reports whether this platform can read a live process working
// directory; declared per platform so an unimplemented one (cwd_other.go)
// skips the watcher entirely.
const cwdTracked = true

// readCwd returns pid's own working directory, or "" when it cannot be read.
func readCwd(pid int) string {
	cwd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return cwd
}

// foregroundPgrp returns the process group that owns pid's controlling
// terminal — tpgid, in /proc/<pid>/stat — or 0 when pid has no terminal or its
// stat cannot be read. Read from the process rather than from the terminal
// (TIOCGPGRP on the PTY master) so no file descriptor has to cross the
// ptyHandle seam for a directory read; macOS answers the same question the
// same way, through sysctl.
func foregroundPgrp(pid int) int {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	return parseTpgid(stat)
}
