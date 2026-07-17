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

// FindBrowser returns the first Chromium-family binary that resolves, trying
// this OS's candidates (candidates_unix.go / candidates_windows.go) in
// preference order. lookPath is injectable for tests (production passes
// exec.LookPath, which also accepts the absolute paths the Windows list uses).
func FindBrowser(lookPath func(name string) (string, error)) (string, error) {
	candidates := browserCandidates()
	for _, name := range candidates {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no chromium-family browser found (tried " +
		fmt.Sprint(candidates) + "); install chromium, chrome or edge")
}

// windowsBrowserCandidates builds the Windows candidate list: chrome, then
// edge (present on every Windows), then brave, each under the install roots
// Windows exposes as environment variables, with bare PATH names last.
// Paths are joined with a literal backslash so the pure logic tests the same
// on any OS. Kept out of the build-tagged file for exactly that reason.
func windowsBrowserCandidates(getenv func(string) string) []string {
	roots := []struct{ env, rel string }{
		{"ProgramFiles", `Google\Chrome\Application\chrome.exe`},
		{"ProgramFiles(x86)", `Google\Chrome\Application\chrome.exe`},
		{"LocalAppData", `Google\Chrome\Application\chrome.exe`},
		{"ProgramFiles(x86)", `Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `BraveSoftware\Brave-Browser\Application\brave.exe`},
	}
	var out []string
	for _, r := range roots {
		if root := getenv(r.env); root != "" {
			out = append(out, root+`\`+r.rel)
		}
	}
	return append(out, "chrome", "msedge")
}

// darwinBrowserCandidates builds the macOS candidate list: chrome, then
// chromium, then edge, then brave, each as its .app executable under the
// system (/Applications) and per-user (~/Applications) install roots, with
// bare PATH names last for a Homebrew-formula install. Paths are joined with a
// literal slash so the pure logic tests the same on any OS. Kept out of the
// build-tagged file for exactly that reason.
func darwinBrowserCandidates(getenv func(string) string) []string {
	apps := []string{
		"Google Chrome.app/Contents/MacOS/Google Chrome",
		"Chromium.app/Contents/MacOS/Chromium",
		"Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"Brave Browser.app/Contents/MacOS/Brave Browser",
	}
	roots := []string{"/Applications"}
	if home := getenv("HOME"); home != "" {
		roots = append(roots, home+"/Applications")
	}
	var out []string
	for _, app := range apps {
		for _, root := range roots {
			out = append(out, root+"/"+app)
		}
	}
	return append(out, "chromium", "google-chrome")
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
// (e.g. --ozone-platform=wayland). onStart, when non-nil, receives the browser
// process once launched, so the caller can close the window itself (the restart
// flow terminates it to relaunch lich); it is called before the blocking wait.
func Run(url, dataDir, class string, extra []string, onStart func(*os.Process)) error {
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
	if onStart != nil {
		onStart(cmd.Process)
	}
	return cmd.Wait()
}
