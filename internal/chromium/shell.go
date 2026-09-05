package chromium

import (
	"os"
	"path/filepath"
)

// shellName is the window binary `task build:shell` produces out of shell/, a
// Rust crate on CEF: lich's own Chromium, launched with the same arguments as
// any browser on the ladder (Args), so nothing about the launch knows which
// one it got.
const shellName = "lich-shell"

// stepShell names the rung the bundled window answers on.
const stepShell = "the bundled window"

// shellPaths lists where the bundled window sits relative to the lich
// executable: beside it (a tarball, bin/ after `task build`), and under the
// lib directory that is sibling to its bin — /usr/local/bin/lich finds
// /usr/local/lib/lich/shell, /usr/bin/lich finds /usr/lib/lich/shell, which
// is where the packages put it.
func shellPaths(exe string) []string {
	dir := filepath.Dir(exe)
	return []string{
		filepath.Join(dir, "shell", shellName),
		filepath.Join(dir, "..", "lib", "lich", "shell", shellName),
	}
}

// findShell returns the first of shellPaths that is a file, or "" when this
// install carries no window of its own.
func findShell(exe string, stat func(string) (os.FileInfo, error)) string {
	for _, path := range shellPaths(exe) {
		if info, err := stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
