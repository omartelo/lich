//go:build darwin

package sandbox

import (
	"slices"
	"strings"
	"testing"
)

func testSpec() Spec {
	return Spec{
		Home:      "/Users/u",
		Cwd:       "/Users/u/work/repo",
		GitCommon: "/Users/u/work/main/.git",
		Write:     []string{"/Users/u/.claude"},
		Read:      []string{"/Users/u/.config"},
	}
}

// lineOf is the index of the first profile line containing needle, or -1.
func lineOf(profile, needle string) int {
	return slices.IndexFunc(strings.Split(profile, "\n"), func(line string) bool {
		return strings.Contains(line, needle)
	})
}

// The last matching rule wins in SBPL, so the two denials have to come before
// the paths they carve exceptions out of. Reversed, the profile parses, runs,
// and confines nothing.
func TestProfileDeniesBeforeItAllows(t *testing.T) {
	profile := seatbeltProfile(testSpec())

	denyWrite := lineOf(profile, "(deny file-write*)")
	denyRead := lineOf(profile, `(deny file-read* (subpath "/Users/u")`)
	if denyWrite < 0 || denyRead < 0 {
		t.Fatalf("a denial is missing:\n%s", profile)
	}
	for _, allowed := range []string{"/Users/u/.config", "/Users/u/.claude", "/Users/u/work/repo"} {
		at := lineOf(profile, allowed)
		if at < 0 {
			t.Errorf("%s is in no rule:\n%s", allowed, profile)
			continue
		}
		if at < denyWrite || at < denyRead {
			t.Errorf("%s is allowed at line %d, before a denial (%d, %d)", allowed, at, denyWrite, denyRead)
		}
	}
}

// Readable is written before writable, so a path in both ends up writable.
func TestProfileGivesTheCheckoutWriteAccess(t *testing.T) {
	profile := seatbeltProfile(testSpec())
	read := lineOf(profile, "(allow file-read* (subpath")
	write := lineOf(profile, "(allow file-read* file-write* (subpath")
	if read < 0 || write < 0 {
		t.Fatalf("an allowance is missing:\n%s", profile)
	}
	if write < read {
		t.Errorf("the writable rule at %d is overridden by the read-only rule at %d", write, read)
	}
	if !strings.Contains(profile, `file-write* (subpath "/Users/u/work/repo")`) {
		t.Errorf("the checkout is not writable:\n%s", profile)
	}
	if !strings.Contains(profile, `(subpath "/Users/u/work/main/.git")`) {
		t.Errorf("the git common directory is not writable:\n%s", profile)
	}
}

// A directory name is something the user typed. A quote in one would close the
// string and continue the policy as code.
func TestProfileEscapesPaths(t *testing.T) {
	spec := testSpec()
	spec.Cwd = `/Users/u/we"ird\path`
	profile := seatbeltProfile(spec)
	if !strings.Contains(profile, `(subpath "/Users/u/we\"ird\\path")`) {
		t.Errorf("a quote or backslash reached the policy unescaped:\n%s", profile)
	}
}

// An operation list with no subpaths is a syntax error, and a profile that
// fails to parse takes the session with it.
func TestProfileOmitsEmptyRules(t *testing.T) {
	profile := seatbeltProfile(Spec{Home: "/Users/u", Cwd: "/Users/u/work/repo"})
	if strings.Contains(profile, "(allow file-read*)") {
		t.Errorf("an empty rule reached the policy:\n%s", profile)
	}
	for _, line := range strings.Split(profile, "\n") {
		if strings.HasSuffix(line, "*") {
			t.Errorf("rule %q names no path:\n%s", line, profile)
		}
	}
}

func TestWrapPassesTheProfileAndTheCommand(t *testing.T) {
	if !Available() {
		t.Skip("sandbox-exec is not installed")
	}
	bin, args := Wrap(testSpec(), "claude", []string{"--model", "opus"})
	if bin != seatbeltBin {
		t.Fatalf("Wrap ran %q, want %q", bin, seatbeltBin)
	}
	if len(args) < 4 || args[0] != "-p" {
		t.Fatalf("no inline profile in %v", args)
	}
	want := []string{"claude", "--model", "opus"}
	if got := args[2:]; !slices.Equal(got, want) {
		t.Errorf("command = %v, want %v", got, want)
	}
}
