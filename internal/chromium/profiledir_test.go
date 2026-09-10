package chromium

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProfileKeyIsOnePerWindow is the whole point of the key: the installed
// window and a pinned dev build never land on the same profile, and one window
// lands on the same profile every time — including across an update, which is
// why the path is not resolved through its symlinks.
func TestProfileKeyIsOnePerWindow(t *testing.T) {
	keys := map[string]string{}
	for _, exe := range []string{
		"/usr/lib/lich/shell/lich-shell",
		"/home/u/lich/bin/shell/lich-shell",
		"/nix/store/abc-lich-0.48.0/lib/lich/shell/lich-shell",
	} {
		key := (Result{Path: exe}).profileKey()
		if other, clash := keys[key]; clash {
			t.Fatalf("%s and %s share the profile key %q", exe, other, key)
		}
		keys[key] = exe
	}
	first := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey()
	if again := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey(); again != first {
		t.Fatalf("profileKey = %q then %q for the same window", first, again)
	}
	if cleaned := (Result{Path: "/usr/lib/lich/shell//lich-shell"}).profileKey(); cleaned != first {
		t.Fatalf("profileKey = %q for an uncleaned path, want %q", cleaned, first)
	}
}

// TestProfileKeyNamesTheWindow keeps the directory readable: whoever opens
// their config directory should be able to tell which profile is whose, on
// every OS — Windows' suffix included. Splitting a Windows path is filepath's
// own job and only its Windows build does it, so the case here is the suffix
// alone.
func TestProfileKeyNamesTheWindow(t *testing.T) {
	cases := map[string]string{
		"/usr/lib/lich/shell/lich-shell":                          "lich-shell-",
		filepath.FromSlash("/lich/shell/lich-shell.exe"):          "lich-shell-",
		filepath.FromSlash("/Lich.app/Contents/MacOS/lich-shell"): "lich-shell-",
	}
	for exe, want := range cases {
		if key := (Result{Path: exe}).profileKey(); !strings.HasPrefix(key, want) {
			t.Errorf("profileKey(%s) = %q, want the %q prefix", exe, key, want)
		}
	}
}

func TestProfileDirKeysTheDirectory(t *testing.T) {
	dir := filepath.FromSlash("/home/u/.config/lich/chromium-profile")
	window := Result{Path: "/usr/lib/lich/shell/lich-shell"}
	want := filepath.Join(dir, window.profileKey())
	if got := window.ProfileDir(dir); got != want {
		t.Fatalf("ProfileDir = %q, want %q", got, want)
	}
}

// unkeyed lays out the profile older lich versions kept directly under the
// root, with a file only its localStorage would have, and returns the root.
func unkeyed(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "chromium-profile")
	if err := os.MkdirAll(filepath.Join(root, profileName, "Local Storage"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(settings(root), []byte("lich.theme"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

// settings names the file standing in for the page's localStorage, relative to
// whichever root it is under.
func settings(root string) string {
	return filepath.Join(root, profileName, "Local Storage", "leveldb")
}

// TestMigrateProfileCarriesTheSettingsOver is the update path: the one profile
// every browser shared becomes the profile of the browser that resolved, with
// the `lich.*` settings in it intact.
func TestMigrateProfileCarriesTheSettingsOver(t *testing.T) {
	root := unkeyed(t)
	key := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey()
	if err := migrateProfile(root, key); err != nil {
		t.Fatalf("migrateProfile: %v", err)
	}
	if _, err := os.Stat(settings(filepath.Join(root, key))); err != nil {
		t.Fatalf("settings did not survive the move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Local State")); err == nil {
		t.Fatal("the unkeyed profile is still at the root of the directory")
	}
	if _, err := os.Stat(root + migratingSuffix); err == nil {
		t.Fatal("the half-moved directory was left behind")
	}
}

// TestMigrateProfileRunsOnce proves the second launch is a no-op rather than a
// second move: by then the root holds keyed directories alone, and one of them
// is a live profile.
func TestMigrateProfileRunsOnce(t *testing.T) {
	root := unkeyed(t)
	key := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey()
	if err := migrateProfile(root, key); err != nil {
		t.Fatalf("first migrateProfile: %v", err)
	}
	if err := migrateProfile(root, key); err != nil {
		t.Fatalf("second migrateProfile: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != key {
		t.Fatalf("root holds %v, want the one keyed profile %q", entries, key)
	}
}

// TestMigrateProfileSkipsWhenTheTargetExists: a profile the browser is already
// using is never overwritten by an older one somebody left at the root.
func TestMigrateProfileSkipsWhenTheTargetExists(t *testing.T) {
	root := unkeyed(t)
	key := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey()
	target := filepath.Join(root, key)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := migrateProfile(root, key); err != nil {
		t.Fatalf("migrateProfile: %v", err)
	}
	if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
		t.Fatalf("target = %v (err %v), want the existing profile untouched", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, "Local State")); err != nil {
		t.Fatalf("the unkeyed profile was moved anyway: %v", err)
	}
}

// TestMigrateProfileFinishesAnInterruptedMove: the move is two renames, so a
// launch that finds the directory parked in between finishes it instead of
// opening on an empty profile and orphaning the settings.
func TestMigrateProfileFinishesAnInterruptedMove(t *testing.T) {
	root := unkeyed(t)
	key := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey()
	if err := os.Rename(root, root+migratingSuffix); err != nil {
		t.Fatalf("park: %v", err)
	}
	if err := migrateProfile(root, key); err != nil {
		t.Fatalf("migrateProfile: %v", err)
	}
	if _, err := os.Stat(settings(filepath.Join(root, key))); err != nil {
		t.Fatalf("settings did not survive the interrupted move: %v", err)
	}
}

// TestMigrateProfileIgnoresAFreshInstall: nothing to move is not an error, and
// a machine that never had the unkeyed profile must not have a directory made
// for it.
func TestMigrateProfileIgnoresAFreshInstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "chromium-profile")
	if err := migrateProfile(root, "lich-shell-0badc0de"); err != nil {
		t.Fatalf("migrateProfile: %v", err)
	}
	if _, err := os.Stat(root); err == nil {
		t.Fatal("migrateProfile created the profile directory on a fresh install")
	}
}

// TestFocusFindsTheDirectoryRunUsed is the duplicate-launch path: focusing a
// running lich hands the URL to a second window process that Chromium's
// profile lock forwards to the first, which only works if both processes
// resolved the same profile directory. Two halves: resolution is a pure
// function of the machine, and the directory Run migrates into is the one
// launch opens.
func TestFocusFindsTheDirectoryRunUsed(t *testing.T) {
	dir := filepath.FromSlash("/home/u/.config/lich/chromium-profile")
	machine := fakeEnv{shell: "/usr/lib/lich/shell/lich-shell"}

	run, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	focus, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if run.ProfileDir(dir) != focus.ProfileDir(dir) {
		t.Fatalf("Focus would open %q, Run opened %q", focus.ProfileDir(dir), run.ProfileDir(dir))
	}
	// What Run migrates into (chromium.go) against what launch opens.
	if got := filepath.Join(dir, run.profileKey()); got != run.ProfileDir(dir) {
		t.Fatalf("migration target %q is not the profile directory %q", got, run.ProfileDir(dir))
	}
}
