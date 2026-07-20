//go:build linux

package terminal

import (
	"os"
	"strconv"
)

// cwdTracked reports whether this platform can read a live process working
// directory. Linux reads /proc; the seam degrades the card to its start path
// elsewhere (macOS would need proc_pidinfo, Windows NtQueryInformationProcess).
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
