package chromium

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// prefsPath is where writePrefs is expected to land, spelled out here so the
// tests fail if the profile layout moves rather than following it.
func prefsPath(dataDir string) string {
	return filepath.Join(dataDir, "Default", "Preferences")
}

func readPrefsFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatalf("prefs are not valid JSON: %v", err)
	}
	return prefs
}

// prefAt walks a nested pref path, failing the test if any hop is missing.
func prefAt(t *testing.T, prefs map[string]any, path ...string) any {
	t.Helper()
	var node any = prefs
	for i, key := range path {
		obj, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%v is not an object at %q", path[:i], key)
		}
		if node, ok = obj[key]; !ok {
			t.Fatalf("pref %v missing", path[:i+1])
		}
	}
	return node
}

// TestWritePrefsSilencesBrowserPrompts is the contract: a fresh profile starts
// with the translate bubble and browser sign-in — the Google account chooser —
// turned off, because neither belongs in a window that is not a browser.
func TestWritePrefsSilencesBrowserPrompts(t *testing.T) {
	dir := t.TempDir()
	if err := writePrefs(dir); err != nil {
		t.Fatalf("writePrefs: %v", err)
	}
	prefs := readPrefsFile(t, prefsPath(dir))

	for _, path := range [][]string{
		{"translate", "enabled"},
		{"signin", "allowed"},
		{"signin", "allowed_on_next_startup"},
	} {
		if got := prefAt(t, prefs, path...); got != false {
			t.Fatalf("pref %v = %v, want false", path, got)
		}
	}
}

// TestWritePrefsKeepsExistingProfile proves the merge is surgical: everything
// Chromium wrote about the profile survives, values lich does not own included,
// and untouched numbers come back exactly as they went in.
func TestWritePrefsKeepsExistingProfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(prefsPath(dir)), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{"browser":{"window_placement":{"bottom":1080}},` +
		`"profile":{"exit_type":"Normal"},` +
		`"signin":{"allowed":true},` +
		`"session":{"startup_id":9007199254740993}}`
	if err := os.WriteFile(prefsPath(dir), []byte(existing), 0o600); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}

	if err := writePrefs(dir); err != nil {
		t.Fatalf("writePrefs: %v", err)
	}
	prefs := readPrefsFile(t, prefsPath(dir))

	if got := prefAt(t, prefs, "browser", "window_placement", "bottom"); got != float64(1080) {
		t.Fatalf("window placement = %v, want it preserved", got)
	}
	if got := prefAt(t, prefs, "profile", "exit_type"); got != "Normal" {
		t.Fatalf("exit_type = %v, want it preserved", got)
	}
	// An existing profile is exactly the case that needs fixing: a lich that
	// already ran once carries the prompts until something turns them off.
	if got := prefAt(t, prefs, "signin", "allowed"); got != false {
		t.Fatalf("signin.allowed = %v, want lich's value to win", got)
	}
	// json.Number, not float64: a 2^53+1 id must survive the round-trip.
	data, err := os.ReadFile(prefsPath(dir))
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	if want := `9007199254740993`; !strings.Contains(string(data), want) {
		t.Fatalf("prefs %s lost the exact number %s", data, want)
	}
}

// TestWritePrefsReplacesNonObject proves a pref path blocked by a scalar is
// rebuilt rather than crashing the launch.
func TestWritePrefsReplacesNonObject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(prefsPath(dir)), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(prefsPath(dir), []byte(`{"translate":"nonsense"}`), 0o600); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}
	if err := writePrefs(dir); err != nil {
		t.Fatalf("writePrefs: %v", err)
	}
	if got := prefAt(t, readPrefsFile(t, prefsPath(dir)), "translate", "enabled"); got != false {
		t.Fatalf("translate.enabled = %v, want false", got)
	}
}

// TestWritePrefsRefusesUnparseableProfile proves a corrupt Preferences file is
// reported and left alone — resetting somebody's profile is worse than the
// bubble the merge was there to silence.
func TestWritePrefsRefusesUnparseableProfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(prefsPath(dir)), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	garbage := []byte("{not json")
	if err := os.WriteFile(prefsPath(dir), garbage, 0o600); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}
	if err := writePrefs(dir); err == nil {
		t.Fatal("want an error for a Preferences file that will not parse")
	}
	data, err := os.ReadFile(prefsPath(dir))
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	if string(data) != string(garbage) {
		t.Fatalf("prefs = %s, want the file left untouched", data)
	}
}

// TestWritePrefsErrorsWhenPrefsUnreadable proves a Preferences path that cannot
// be read at all is reported rather than replaced with a fresh profile.
func TestWritePrefsErrorsWhenPrefsUnreadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(prefsPath(dir), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writePrefs(dir); err == nil {
		t.Fatal("want an error when Preferences cannot be read")
	}
}

// TestWritePrefsErrorsWhenProfileDirBlocked proves an unusable data dir surfaces
// as an error instead of a silent no-op.
func TestWritePrefsErrorsWhenProfileDirBlocked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := writePrefs(dir); err == nil {
		t.Fatal("want an error when the profile dir cannot be created")
	}
}
