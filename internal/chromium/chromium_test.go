package chromium

import (
	"errors"
	"slices"
	"testing"
)

// TestFindBrowserPicksFirstHit proves missing candidates are skipped and
// preference order decides among the installed ones — against whichever
// candidate list this OS compiles in.
func TestFindBrowserPicksFirstHit(t *testing.T) {
	candidates := browserCandidates()
	if len(candidates) < 2 {
		t.Fatal("candidate list too short to prove ordering")
	}
	hits := map[string]bool{candidates[1]: true, candidates[len(candidates)-1]: true}
	lookPath := func(name string) (string, error) {
		if hits[name] {
			return "/resolved/" + name, nil
		}
		return "", errors.New("not found")
	}
	got, err := FindBrowser(lookPath)
	if err != nil {
		t.Fatalf("FindBrowser: %v", err)
	}
	if want := "/resolved/" + candidates[1]; got != want {
		t.Fatalf("FindBrowser = %q, want the earliest installed candidate %q", got, want)
	}
}

func TestFindBrowserErrorsWhenNoneInstalled(t *testing.T) {
	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	if _, err := FindBrowser(lookPath); err == nil {
		t.Fatal("want error when no browser is on PATH")
	}
}

// TestWindowsBrowserCandidates proves the Windows list expands only the
// install roots present in the environment, prefers chrome > edge > brave,
// and always ends with the bare PATH names as a last resort.
func TestWindowsBrowserCandidates(t *testing.T) {
	env := map[string]string{
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
	}
	got := windowsBrowserCandidates(func(k string) string { return env[k] })

	want := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		"chrome",
		"msedge",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
}

// TestWindowsBrowserCandidatesEmptyEnv proves a bare environment still leaves
// the PATH names, so FindBrowser never iterates an empty list.
func TestWindowsBrowserCandidatesEmptyEnv(t *testing.T) {
	got := windowsBrowserCandidates(func(string) string { return "" })
	if !slices.Equal(got, []string{"chrome", "msedge"}) {
		t.Fatalf("candidates = %v, want PATH names only", got)
	}
}

// TestDarwinBrowserCandidates proves the macOS list expands each browser under
// both the system and per-user Applications roots, prefers chrome > chromium >
// edge > brave, and ends with the bare PATH names.
func TestDarwinBrowserCandidates(t *testing.T) {
	got := darwinBrowserCandidates(func(k string) string {
		if k == "HOME" {
			return "/Users/u"
		}
		return ""
	})

	want := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Users/u/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Users/u/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Users/u/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Users/u/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"chromium",
		"google-chrome",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
}

// TestDarwinBrowserCandidatesNoHome proves a missing HOME drops the per-user
// root but still leaves the system paths and PATH names, so FindBrowser never
// iterates an empty list.
func TestDarwinBrowserCandidatesNoHome(t *testing.T) {
	got := darwinBrowserCandidates(func(string) string { return "" })
	want := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"chromium",
		"google-chrome",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
}

func TestArgs(t *testing.T) {
	args := Args("http://127.0.0.1:47821/?token=x", "/home/u/.config/lich/chromium-profile", "lichdev", []string{"--ozone-platform=wayland"})
	for _, want := range []string{
		"--app=http://127.0.0.1:47821/?token=x",
		"--user-data-dir=/home/u/.config/lich/chromium-profile",
		"--class=lichdev",
		"--ozone-platform=wayland",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	if args[len(args)-1] != "--ozone-platform=wayland" {
		t.Fatalf("extra args must come last: %v", args)
	}
}
