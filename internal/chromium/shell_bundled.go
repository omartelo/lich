//go:build linux || windows || (darwin && arm64)

package chromium

import "os"

// bundledShell is the window lich ships, found relative to its own executable.
// Empty under `go run` or for a binary copied out on its own — which is what
// the LICH_SHELL pin exists for.
func bundledShell() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return findShell(exe, os.Stat)
}
