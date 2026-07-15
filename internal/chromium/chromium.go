// Package chromium launches the app window as a system Chromium in --app
// mode, pointed at the loopback listener that serves the frontend and the
// RPC/terminal transports — option 1 of docs/chromium-shell.md. Promoted from
// the cmd/spike skeleton that validated the approach.
package chromium

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Candidate binaries, in preference order. Any Chromium gives the same
// compositor; the preference only picks the most conventional install.
var browserCandidates = []string{
	"chromium",
	"chromium-browser",
	"google-chrome-stable",
	"google-chrome",
	"brave",
}

// FindBrowser returns the first Chromium-family binary on PATH. lookPath is
// injectable for tests (production passes exec.LookPath).
func FindBrowser(lookPath func(name string) (string, error)) (string, error) {
	for _, name := range browserCandidates {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no chromium-family browser found on PATH (tried " +
		fmt.Sprint(browserCandidates) + "); install chromium")
}

// Args builds the --app invocation. The dedicated user-data-dir is
// load-bearing twice over: without it Chromium adopts the window into an
// already-running instance (the spawned process exits immediately, breaking
// the window-closed-means-quit lifecycle), and the profile holds the
// frontend's localStorage (lich.* settings), so it must persist across runs.
// class is the WM_CLASS: the dev shell passes its own so compositor window
// rules targeting the daily driver never capture the dev window.
func Args(url, dataDir, class string, extra []string) []string {
	args := []string{
		"--app=" + url,
		"--user-data-dir=" + dataDir,
		"--class=" + class,
		"--no-first-run",
		"--no-default-browser-check",
	}
	return append(args, extra...)
}

// Run opens the window and blocks until the user closes it — the browser
// process exiting is the app lifecycle. Extra args pass through to Chromium
// (e.g. --ozone-platform=wayland).
func Run(url, dataDir, class string, extra []string) error {
	browser, err := FindBrowser(exec.LookPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("chromium profile dir: %w", err)
	}
	cmd := exec.Command(browser, Args(url, dataDir, class, extra)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", browser, err)
	}
	return cmd.Wait()
}
