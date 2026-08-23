//go:build linux

package terminal

import (
	"os"
	"strconv"
	"strings"
)

// envReadable reports whether this platform can read the environment of a
// process lich started; declared per platform the way cwdTracked is, so the
// callers on a platform without it answer "cannot tell" rather than guessing.
const envReadable = true

// readEnv returns the environment pid runs with, or nil when it cannot be read
// (the process exited, or was never ours to inspect).
//
// /proc holds the environment of the process's current image, not of the one
// lich spawned — and that difference is the whole point. A wrapper binary
// exports a config dir or a token and then `exec`s the provider, replacing
// itself; what shows up here is what the provider actually runs with, which is
// where a session's credentials come from.
func readEnv(pid int) map[string]string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return nil
	}
	env := make(map[string]string)
	for entry := range strings.SplitSeq(string(data), "\x00") {
		if key, value, ok := strings.Cut(entry, "="); ok {
			env[key] = value
		}
	}
	return env
}
