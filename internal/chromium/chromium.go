// Package chromium launches the app window as a system Chromium in --app
// mode, pointed at the loopback listener that serves the frontend and the
// RPC/terminal transports — option 1 of docs/chromium-shell.md.
package chromium

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// windowsBrowserCandidates builds the Windows candidate list: chrome, then
// edge (present on every Windows), then brave and vivaldi, each under the
// install roots Windows exposes as environment variables, with bare PATH names
// last. Paths are joined with a literal backslash so the pure logic tests the
// same on any OS. Kept out of the build-tagged file for exactly that reason.
//
// No registry read behind it: StartMenuInternet would name a browser installed
// somewhere else entirely, and reaching for it costs a dependency for the
// install nobody has. LICH_BROWSER covers that machine in one variable.
func windowsBrowserCandidates(getenv func(string) string) []string {
	roots := []struct{ env, rel string }{
		{"ProgramFiles", `Google\Chrome\Application\chrome.exe`},
		{"ProgramFiles(x86)", `Google\Chrome\Application\chrome.exe`},
		{"LocalAppData", `Google\Chrome\Application\chrome.exe`},
		{"ProgramFiles(x86)", `Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `Microsoft\Edge\Application\msedge.exe`},
		{"ProgramFiles", `BraveSoftware\Brave-Browser\Application\brave.exe`},
		{"LocalAppData", `BraveSoftware\Brave-Browser\Application\brave.exe`},
		{"ProgramFiles", `Vivaldi\Application\vivaldi.exe`},
		{"LocalAppData", `Vivaldi\Application\vivaldi.exe`},
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
// chromium, then edge, then brave, then vivaldi, each as its .app executable under the
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
		"Vivaldi.app/Contents/MacOS/Vivaldi",
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
		// Naming the profile is also what keeps the profile picker shut:
		// Chromium documents the picker as suppressed whenever the command line
		// names a profile directory. Without it, a Chrome that knows several
		// Google accounts can open the app on an account chooser.
		"--profile-directory=" + profileName,
		"--class=" + class,
		"--no-first-run",
		"--no-default-browser-check",
		// The translate bubble compares the page's language against the browser
		// locale and offers to translate lich's own UI — a browser prompt in a
		// window that is not a browser. The profile pref of the same name
		// (profile.go) is what actually holds it down; this only asks.
		"--disable-features=Translate",
	}
	return append(args, extra...)
}

// Run opens the window and blocks until the user closes it — the browser
// process exiting is the app lifecycle. The browser is whatever Resolve found;
// ErrNoBrowser comes back untouched, because the answer to a machine with no
// Chromium-family browser on it is not this function's to give. Extra args pass
// through to Chromium (e.g. --ozone-platform=wayland). onStart, when non-nil,
// receives the browser process once launched, so the caller can close the window
// itself (the restart flow terminates it to relaunch lich); it is called before
// the blocking wait.
func Run(url, dataDir, class string, extra []string, onStart func(*os.Process)) error {
	browser, err := Resolve(RealEnv())
	if err != nil {
		return err
	}
	slog.Info("browser resolved", "browser", browser.Describe())
	dataDir = browser.ProfileDir(dataDir)
	// The data dir is required to launch at all; the prefs inside it only
	// decide how quiet the window is.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("chromium profile dir: %w", err)
	}
	if err := writePrefs(dataDir); err != nil {
		// A profile lich could not pre-configure is a browser that may ask
		// about translation or Google accounts. Annoying; not a reason to
		// refuse to open the window.
		slog.Warn("chromium profile prefs", "err", err)
	}
	argv := append(append([]string{}, browser.Prefix...), Args(url, dataDir, class, extra)...)
	cmd := exec.Command(browser.Path, argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", browser.Path, err)
	}
	if onStart != nil {
		onStart(cmd.Process)
	}
	return cmd.Wait()
}
