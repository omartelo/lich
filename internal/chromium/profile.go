package chromium

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// profileName is the profile directory lich uses inside its user-data-dir. It
// is Chromium's own default name; naming it explicitly is what matters (see
// Args), so the name and the --profile-directory switch share this constant.
const profileName = "Default"

// appPrefs are the preferences lich owns in its Chromium profile. Each one
// silences a prompt Chromium raises over the window as if it were a browser —
// the two a stranger's first launch met on Windows.
//
// They are preferences rather than command-line switches because a switch only
// asks at launch, in whatever vocabulary that Chromium build happens to speak:
// --disable-features=Translate is one feature rename away from being ignored,
// while translate.enabled is the toggle the Settings page writes and every
// build honours. The switches stay too — belt and braces on a surface neither
// of us can test on every browser a user might have installed.
var appPrefs = []struct {
	path  []string
	value any
}{
	// The bubble offering to translate lich's own UI into the browser locale.
	{[]string{"translate", "enabled"}, false},
	// The Google account chooser Chrome opens on a profile it considers new —
	// on a machine whose Chrome knows several accounts, that chooser is the
	// first thing the app shows. A window that is not a browser has no account
	// to sign in to, and --app mode offers no way to sign in anyway.
	// allowed_on_next_startup is the desktop toggle that feeds signin.allowed
	// on the following launch; both are set so the prompt cannot return on the
	// second run.
	{[]string{"signin", "allowed"}, false},
	{[]string{"signin", "allowed_on_next_startup"}, false},
}

// writePrefs merges appPrefs into the profile's Preferences file, creating the
// profile when it is new and leaving every other key of an existing one
// untouched. Chromium reads that file at startup and rewrites it at exit, so
// the merge has to happen before the browser launches — and it happens on every
// launch, not only the first: a profile created by an older lich already
// carries the prompts, and nobody is going to delete their profile to lose an
// account chooser.
func writePrefs(dataDir string) error {
	profileDir := filepath.Join(dataDir, profileName)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("chromium profile dir: %w", err)
	}
	path := filepath.Join(profileDir, "Preferences")
	prefs, err := readPrefs(path)
	if err != nil {
		return err
	}
	for _, pref := range appPrefs {
		setPref(prefs, pref.path, pref.value)
	}
	return writePrefsFile(path, prefs)
}

// readPrefs decodes an existing Preferences file, or returns an empty object
// when the profile has none yet. Numbers stay json.Number so the round-trip
// hands back every value lich does not understand exactly as Chromium wrote it.
// A file that will not parse is returned as an error and never overwritten:
// silencing a bubble is not worth resetting somebody's profile.
func readPrefs(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read chromium prefs: %w", err)
	}
	prefs := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&prefs); err != nil {
		return nil, fmt.Errorf("parse chromium prefs %s: %w", path, err)
	}
	return prefs, nil
}

// setPref writes value at a pref path, creating the intermediate objects
// Chromium's nested Preferences JSON uses. Anything that is not an object in
// the way is replaced: lich's prefs win over a shape they cannot merge into.
func setPref(root map[string]any, path []string, value any) {
	node := root
	for _, key := range path[:len(path)-1] {
		child, ok := node[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[key] = child
		}
		node = child
	}
	node[path[len(path)-1]] = value
}

// writePrefsFile replaces the Preferences file in one step. Chromium writes it
// the same way, and a half-written profile is a browser that refuses to open.
func writePrefsFile(path string, prefs map[string]any) error {
	data, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("encode chromium prefs: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "prefs-*")
	if err != nil {
		return fmt.Errorf("write chromium prefs: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write chromium prefs: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write chromium prefs: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write chromium prefs: %w", err)
	}
	return nil
}
