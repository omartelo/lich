package chromium

import (
	"os"
	"path/filepath"
)

// shellName is the window binary `task build:shell` produces out of shell/, a
// Rust crate on CEF: lich's own Chromium, launched with the same arguments as
// any browser on the ladder (Args) plus exitOnStdinEOF, the one switch that is
// its own. The executable suffix is the only thing about it Windows changes
// (shell_name_windows.go).
const shellName = "lich-shell" + shellExt

// stepShell names the rung the bundled window answers on.
const stepShell = "the bundled window"

// exitOnStdinEOF is the switch that tells the bundled window to end when its
// stdin does (shell/src/main.rs). Passed to it alone: a system browser has no
// such switch, and lich holds no pipe to one.
const exitOnStdinEOF = "--exit-on-stdin-eof"

// shellPaths lists where the bundled window sits relative to the lich
// executable: under shell/ beside it (a tarball, bin/ after `task build`),
// directly beside it (Lich.app's Contents/MacOS, where a process has to sit
// for macOS to count it as the app's own, docs/chromium-shell.md), and under
// the lib directory that is sibling to its bin — /usr/local/bin/lich finds
// /usr/local/lib/lich/shell, /usr/bin/lich finds /usr/lib/lich/shell, which
// is where the packages put it.
func shellPaths(exe string) []string {
	dir := filepath.Dir(exe)
	return []string{
		filepath.Join(dir, "shell", shellName),
		filepath.Join(dir, shellName),
		filepath.Join(dir, "..", "lib", "lich", "shell", shellName),
	}
}

// findShell returns the first of shellPaths that is a file, or "" when this
// install carries no window of its own.
//
// The executable is resolved through its symlinks first: the Homebrew cask puts
// `lich` on PATH as a link into Lich.app and os.Executable hands back the link,
// not its target, so the window would be looked for beside /opt/homebrew/bin and
// never found — a bundled install launched by name would open a system browser
// while the one launched by its icon opened lich's own window. Left as written
// when it resolves to nothing, which is every path a test names.
func findShell(exe string, stat func(string) (os.FileInfo, error)) string {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	for _, path := range shellPaths(exe) {
		if info, err := stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
