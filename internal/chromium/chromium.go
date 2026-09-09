// Package chromium launches the app window: lich's own Chromium (CEF, built
// out of shell/), pointed at the loopback listener that serves the frontend
// and the RPC/terminal transports (docs/chromium-shell.md).
package chromium

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// Args builds the window's argv; shell/src/main.rs is the other side of the
// contract. The dedicated user-data-dir is load-bearing: the profile holds the
// frontend's localStorage (lich.* settings), so it must persist across runs.
// class is the WM_CLASS: the dev shell passes its own so compositor window
// rules targeting the daily driver never capture the dev window.
func Args(url, dataDir, class string, extra []string) []string {
	args := []string{
		"--url=" + url,
		"--user-data-dir=" + dataDir,
		"--class=" + class,
		// CEF's Chrome runtime still carries Chrome's first-run and
		// default-browser prompts; these are the switches Chrome documents
		// to hold them down.
		"--no-first-run",
		"--no-default-browser-check",
		// The translate bubble compares the page's language against the
		// locale and offers to translate lich's own UI. CEF reads feature
		// lists straight off argv (shell/src/main.rs).
		"--disable-features=Translate",
		exitOnStdinEOF,
	}
	return append(args, extra...)
}

// startupGrace is how long after launch the bundled window's death still counts
// as failing to open — a segfault on first paint, a system library libcef.so
// cannot find on this distribution — rather than as the window's lifecycle
// ending. The user closing it exits 0 and never trips this; a crash an hour in
// is the exit it always was. Half a minute rather than the ten seconds a crash
// itself takes: a segfault is only reported once its core dump is written, and
// systemd-coredump took 18 s over the window's 1.3 GB tree (measured), so a
// crash one second in reached Wait at nineteen and was read as a closed window.
const startupGrace = 30 * time.Second

// Run opens the window and blocks until the user closes it — the window
// process exiting is the app lifecycle. ErrNoShell comes back untouched,
// because the answer to an install with no window is not this function's to
// give. Extra args pass through to Chromium (e.g. --ozone-platform=wayland).
// onStart, when non-nil, receives the window process once launched, so the
// caller can close the window itself (the restart flow terminates it to
// relaunch lich); it is called before the blocking wait.
func Run(url, dataDir, class string, extra []string, onStart func(*os.Process)) error {
	start := func(window Result) error {
		// Here and not in launch: Focus resolves the same directory against a
		// lich that has already done this, and renaming a profile a running
		// window holds open is not a migration.
		if err := migrateProfile(dataDir, window.profileKey()); err != nil {
			// The profile is only the user's settings; a launch that could not
			// carry them over still opens a window.
			slog.Warn("chromium profile migration", "err", err)
		}
		return launch(window, url, dataDir, class, extra, onStart)
	}
	return run(RealEnv(), start, TabFallback)
}

// Focus hands url to the window a running lich has open and waits for the
// hand-off to end. The process is a second instance that Chromium's profile
// lock forwards to the first, and its exit is not a window failing to open —
// CEF reports the forward as a failed initialise, so the duplicate exits 1 by
// design.
func Focus(url, dataDir, class string) error {
	window, err := Resolve(RealEnv())
	if err != nil {
		return err
	}
	return launch(window, url, dataDir, class, nil, nil)
}

// run is Run with the launch and the platform's fallback as seams, so the
// startup fallback is testable without a window, on every OS.
func run(env Env, start func(Result) error, tabFallback bool) error {
	window, err := Resolve(env)
	if err != nil {
		return err
	}
	slog.Info("window resolved", "window", window.Describe())
	started := time.Now()
	err = start(window)
	if tabFallback && fallsBack(window.Step, err, time.Since(started)) {
		// The crash itself is here, in the log; ErrNoShell reaches main.go's
		// tab path, which keeps lich reachable.
		slog.Error("bundled window died at startup, opening a tab instead", "err", err)
		return ErrNoShell
	}
	return err
}

// fallsBack is whether a launch that has ended is the bundled window failing to
// open: only the bundled window (a pin is the user's word), only an error exit,
// and only within startupGrace.
func fallsBack(step string, err error, elapsed time.Duration) bool {
	return step == stepShell && err != nil && elapsed < startupGrace
}

// launch starts the resolved window on the profile and waits for it to exit.
func launch(window Result, url, dataDir, class string, extra []string, onStart func(*os.Process)) error {
	dataDir = window.ProfileDir(dataDir)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("chromium profile dir: %w", err)
	}
	cmd := exec.Command(window.Path, Args(url, dataDir, class, extra)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// The window's life is tied to this process by a pipe it reads for EOF: a
	// lich that dies without closing its window (a kill, an out-of-memory, a
	// crash) closes the write end with it, and the window goes. Left running,
	// it was the orphan the next launch's window was forwarded to by CEF's
	// process singleton. Only the write end is held here; Go marks both ends
	// close-on-exec, so no session inherits it.
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("launch %s: %w", window.Path, err)
	}
	defer w.Close()
	defer r.Close()
	cmd.Stdin = r
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", window.Path, err)
	}
	if onStart != nil {
		onStart(cmd.Process)
	}
	return cmd.Wait()
}
