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

// processCwd returns pid's current working directory, or "" when it cannot be
// read (the process exited, or was never ours to inspect).
func processCwd(pid int) string {
	cwd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return cwd
}
