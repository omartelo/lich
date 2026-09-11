package chromium

import (
	"errors"
	"fmt"
)

// ErrNoBrowser is a machine with no Chromium-family browser the agent sidecar
// can drive. Distinct from ErrNoShell: the UI window is lich-shell; this is
// Chrome/Edge/Chromium/… for CDP (internal/browser).
var ErrNoBrowser = errors.New(
	"no chromium-family browser found — install chromium, chrome, brave, vivaldi or edge")

// FindBrowser returns the first Chromium-family binary that resolves, trying
// this OS's candidates (candidates_*.go) in preference order. lookPath is
// injectable for tests (production passes exec.LookPath, which also accepts
// the absolute paths the Windows and macOS lists use).
//
// This is the agent-browser sidecar only — never the lich UI window, which
// Resolve pins to LICH_SHELL / lich-shell (#570).
func FindBrowser(lookPath func(name string) (string, error)) (string, error) {
	candidates := browserCandidates()
	for _, name := range candidates {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w (tried %v)", ErrNoBrowser, candidates)
}

// chromiumNames are the Chromium-family executables on Linux/BSD PATH, in
// preference order. candidates_unix.go returns this list; Windows and macOS
// build absolute paths under the install roots instead.
var chromiumNames = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"google-chrome-beta",
	"helium-browser",
	"brave",
	"brave-browser",
	"vivaldi",
	"vivaldi-stable",
	"microsoft-edge",
	"microsoft-edge-stable",
	"thorium-browser",
	"ungoogled-chromium",
	"chrome",
}

// windowsBrowserCandidates builds the Windows candidate list: chrome, then
// edge (present on every Windows), then brave and vivaldi, each under the
// install roots Windows exposes as environment variables, with bare PATH names
// last. Paths are joined with a literal backslash so the pure logic tests the
// same on any OS. Kept out of the build-tagged file for exactly that reason.
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
// chromium, then edge, then brave, then vivaldi, each as its .app executable
// under the system (/Applications) and per-user (~/Applications) install roots,
// with bare PATH names last for a Homebrew-formula install. Paths are joined
// with a literal slash so the pure logic tests the same on any OS.
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
