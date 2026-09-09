package chromium

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// profileName is the profile directory Chromium keeps inside its user-data-dir
// when none is named on the command line. It is what an unkeyed profile from
// an older lich holds (unkeyedProfile).
const profileName = "Default"

// keyLen is how many hex characters of the digest name a profile directory. The
// set being keyed is the window builds one machine launches — the package's and
// a `task dev` pin — so 32 bits is already far more than the collision needs,
// and the rest of the digest would only lengthen a name a human has to read in
// their config directory.
const keyLen = 8

// migratingSuffix names the old profile while it is being moved. The move
// cannot be a single rename — a directory will not go inside itself — so it
// steps out to this sibling first, and a launch that finds one left behind
// finishes the move instead of opening on an empty profile.
const migratingSuffix = ".moving"

// profileKey names the profile directory this resolution owns: the window's
// own name, so the config directory still reads, and a short digest of its
// path, so a pinned dev window never shares a profile with the installed one.
// The digest is of the path alone, which is what every profile on disk today
// was keyed by when a system browser could still be the window.
//
// The path is cleaned and deliberately *not* resolved through its symlinks. On
// Nix, and on any store-style install, the real executable sits under a path
// carrying its version, so keying on that would hand the user a brand new
// profile — an empty UI, every `lich.*` setting gone — on each update. The
// path the ladder resolved is the stable name for the same window.
func (r Result) profileKey() string {
	exe := filepath.Clean(r.Path)
	sum := sha256.Sum256([]byte(exe))
	return exeName(exe) + "-" + hex.EncodeToString(sum[:])[:keyLen]
}

// exeName reduces the executable's name to what is a directory name everywhere
// lich runs: lowercased, without Windows' suffix, everything outside [a-z0-9]
// folded to a dash.
func exeName(exe string) string {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(exe), ".exe"))
	return strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, base), "-")
}

// ProfileDir is where this window's profile goes, given the directory lich
// would use: one keyed subdirectory per window. A Chromium profile is a
// particular build's own state, so handing one build's to another is a silent
// adoption at best, and at worst a window that refuses to start on a profile a
// newer one wrote.
func (r Result) ProfileDir(dir string) string {
	return filepath.Join(dir, r.profileKey())
}

// migrateProfile moves the single unkeyed profile older lich versions kept
// directly under root into the subdirectory the window resolved at this launch
// now owns. It happens once: afterwards root holds keyed directories alone and
// the check below finds nothing to move. What is being carried over is the
// page's localStorage — every `lich.*` UI setting lives in that profile — so
// skipping it would be a window that came back factory-fresh after an update.
func migrateProfile(root, key string) error {
	target := filepath.Join(root, key)
	moving := root + migratingSuffix
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if _, err := os.Stat(moving); err != nil {
		if !unkeyedProfile(root) {
			return nil
		}
		if err := os.Rename(root, moving); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	return os.Rename(moving, target)
}

// unkeyedProfile reports whether root is itself a Chromium profile rather than
// the directory of profiles it is now. `Local State` is the file every Chromium
// writes into its user-data-dir and `Default` the profile directory lich names
// on the command line (Args); the keyed layout has neither at its root.
func unkeyedProfile(root string) bool {
	for _, name := range []string{"Local State", profileName} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}
