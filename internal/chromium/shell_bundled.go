//go:build linux || windows || (darwin && arm64)

package chromium

import "os"

// shellExpected: the Linux packages, the Windows installer and the Apple
// Silicon Lich.app ship the window, so opening a system browser here is a
// fallback worth a warning, not the design.
const shellExpected = true

// bundledShell is the window lich ships, found relative to its own executable.
// Empty under `go run` or for a binary copied out on its own — the one case
// the rungs below it still answer.
func bundledShell() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return findShell(exe, os.Stat)
}
