package chromium

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProfileKeyIsOnePerBrowser is the whole point of the key: two browsers on
// one machine never land on the same profile, and one browser lands on the same
// profile every time — including across an update, which is why the path is not
// resolved through its symlinks.
func TestProfileKeyIsOnePerBrowser(t *testing.T) {
	keys := map[string]string{}
	for _, exe := range []string{
		"/usr/bin/chromium",
		"/usr/bin/helium-browser",
		"/usr/lib/lich/shell/lich-shell",
		"/run/current-system/sw/bin/chromium",
	} {
		key := (Result{Path: exe}).profileKey()
		if other, clash := keys[key]; clash {
			t.Fatalf("%s and %s share the profile key %q", exe, other, key)
		}
		keys[key] = exe
	}
	first := (Result{Path: "/usr/bin/chromium"}).profileKey()
	if again := (Result{Path: "/usr/bin/chromium"}).profileKey(); again != first {
		t.Fatalf("profileKey = %q then %q for the same browser", first, again)
	}
	if cleaned := (Result{Path: "/usr/bin//chromium"}).profileKey(); cleaned != first {
		t.Fatalf("profileKey = %q for an uncleaned path, want %q", cleaned, first)
	}
}

// TestProfileKeyDividesFlatpaks pins the reason the key is the whole command
// and not the executable: every Chromium-family Flatpak is launched through the
// same `flatpak` binary, so keying on the path alone would put all of them back
// on one profile.
func TestProfileKeyDividesFlatpaks(t *testing.T) {
	chromium := Result{Path: "/usr/bin/flatpak", Prefix: []string{"run", "org.chromium.Chromium"}}
	brave := Result{Path: "/usr/bin/flatpak", Prefix: []string{"run", "com.brave.Browser"}}
	if chromium.profileKey() == brave.profileKey() {
		t.Fatalf("two Flatpak browsers share the profile key %q", chromium.profileKey())
	}
}

// TestProfileKeyNamesTheBrowser keeps the directory readable: whoever opens
// their config directory should be able to tell which profile is whose, in
// every spelling of an executable the candidate lists hold — Windows' suffix
// and the spaces in a macOS bundle among them. Splitting a Windows path is
// filepath's own job and only its Windows build does it, so the case here is
// the suffix alone.
func TestProfileKeyNamesTheBrowser(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/chromium": "chromium-",
		filepath.FromSlash("/Google/Chrome/Application/chrome.exe"):                        "chrome-",
		filepath.FromSlash("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"): "google-chrome-",
	}
	for exe, want := range cases {
		if key := (Result{Path: exe}).profileKey(); !strings.HasPrefix(key, want) {
			t.Errorf("profileKey(%s) = %q, want the %q prefix", exe, key, want)
		}
	}
}

// TestProfileDirKeysTheDirectory covers both shapes: the directory lich chose,
// and the one a Flatpak's sandbox can actually write.
func TestProfileDirKeysTheDirectory(t *testing.T) {
	dir := filepath.FromSlash("/home/u/.config/lich/chromium-profile")
	browser := Result{Path: "/usr/bin/chromium"}
	want := filepath.Join(dir, browser.profileKey())
	if got := browser.ProfileDir(dir); got != want {
		t.Fatalf("ProfileDir = %q, want %q", got, want)
	}

	flatpak := Result{
		Path:        "/usr/bin/flatpak",
		Prefix:      []string{"run", "com.vivaldi.Vivaldi"},
		ProfileRoot: "/home/u/.var/app/com.vivaldi.Vivaldi/config",
	}
	// Slashes even when this test runs on Windows: a Flatpak path is a Linux
	// path whatever the binary was built for.
	wantFlatpak := "/home/u/.var/app/com.vivaldi.Vivaldi/config/chromium-profile-dev/" + flatpak.profileKey()
	if got := flatpak.ProfileDir("/home/u/.config/lich/chromium-profile-dev"); got != wantFlatpak {
		t.Fatalf("ProfileDir = %q, want %q", got, wantFlatpak)
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
	key := (Result{Path: "/usr/bin/chromium"}).profileKey()
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
	key := (Result{Path: "/usr/bin/chromium"}).profileKey()
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
	key := (Result{Path: "/usr/bin/chromium"}).profileKey()
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
	key := (Result{Path: "/usr/bin/chromium"}).profileKey()
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
	if err := migrateProfile(root, "chromium-0badc0de"); err != nil {
		t.Fatalf("migrateProfile: %v", err)
	}
	if _, err := os.Stat(root); err == nil {
		t.Fatal("migrateProfile created the profile directory on a fresh install")
	}
}

// TestFocusFindsTheDirectoryRunUsed is the duplicate-launch path: focusing a
// running lich hands the URL to a second browser process that Chromium's
// profile lock forwards to the first, which only works if both processes
// resolved the same profile directory. Two halves: resolution is a pure
// function of the machine, and the directory Run migrates into is the one
// launch opens.
func TestFocusFindsTheDirectoryRunUsed(t *testing.T) {
	dir := filepath.FromSlash("/home/u/.config/lich/chromium-profile")
	machine := fakeEnv{installed: map[string]bool{"chromium": true, "vivaldi": true}}

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
	if got := filepath.Join(run.relocate(dir), run.profileKey()); got != run.ProfileDir(dir) {
		t.Fatalf("migration target %q is not the profile directory %q", got, run.ProfileDir(dir))
	}
}
