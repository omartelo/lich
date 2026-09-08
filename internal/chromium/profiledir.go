package chromium

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// keyLen is how many hex characters of the digest name a profile directory. The
// set being keyed is the Chromium-family browsers installed on one machine — a
// handful — so 32 bits is already far more than the collision needs, and the
// rest of the digest would only lengthen a name a human has to read in their
// config directory.
const keyLen = 8

// migratingSuffix names the old profile while it is being moved. The move
// cannot be a single rename — a directory will not go inside itself — so it
// steps out to this sibling first, and a launch that finds one left behind
// finishes the move instead of opening on an empty profile.
const migratingSuffix = ".moving"

// profileKey names the profile directory this resolution owns: the browser's
// own name, so the config directory still reads, and a short digest of the
// exact command behind it, so no two browsers ever share a profile. The whole
// command and not the path alone, because every Chromium-family Flatpak runs
// through the same `flatpak` binary.
//
// The path is cleaned and deliberately *not* resolved through its symlinks. On
// Nix, and on any store-style install, the real executable sits under a path
// carrying its version, so keying on that would hand the user a brand new
// profile — an empty UI, every `lich.*` setting gone — on each browser update.
// The launcher the ladder resolved is the stable name for the same browser.
func (r Result) profileKey() string {
	exe := filepath.Clean(r.Path)
	command := strings.Join(append([]string{exe}, r.Prefix...), " ")
	sum := sha256.Sum256([]byte(command))
	return browserName(exe) + "-" + hex.EncodeToString(sum[:])[:keyLen]
}

// browserName reduces the executable's name to what is a directory name
// everywhere lich runs: lowercased, without Windows' suffix, everything outside
// [a-z0-9] folded to a dash ("Google Chrome.app/Contents/MacOS/Google Chrome"
// is a macOS candidate, spaces and all).
func browserName(exe string) string {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(exe), ".exe"))
	return strings.Trim(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, base), "-")
}

// relocate is dir where this browser can actually write: only a sandboxed one
// moves, and it keeps the directory's name so the dev shell still gets a tree
// of its own.
//
// Slashes, not filepath: a ProfileRoot is only ever set by the Flatpak rung,
// and a Flatpak path is a Linux path whatever this binary was built for.
// filepath here would follow the *build* OS and hand a Linux sandbox a
// backslash-joined directory — the same trap the two candidate builders in
// chromium.go join by hand to stay out of.
func (r Result) relocate(dir string) string {
	if r.ProfileRoot == "" {
		return dir
	}
	return r.ProfileRoot + "/" + path.Base(dir)
}

// ProfileDir is where this browser's profile goes, given the directory lich
// would use: one keyed subdirectory per browser. A Chromium profile is a
// particular build's own state, so handing one browser's to another is a silent
// adoption at best, and at worst a browser that refuses to start on a profile a
// newer one wrote.
func (r Result) ProfileDir(dir string) string {
	if r.ProfileRoot != "" {
		return r.relocate(dir) + "/" + r.profileKey()
	}
	return filepath.Join(dir, r.profileKey())
}

// migrateProfile moves the single unkeyed profile older lich versions kept
// directly under root into the subdirectory the browser resolved at this launch
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
