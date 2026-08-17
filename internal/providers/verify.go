package providers

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// A Check's Status. Each failing value names a distinct mistake, because each
// one is silent today: the session simply refuses to start, and the settings
// screen that accepted the value says nothing.
const (
	// CheckOK: the binary resolves and can be executed.
	CheckOK = "ok"
	// CheckEmpty: nothing is configured at this layer, so the next one answers.
	CheckEmpty    = "empty"
	CheckNotFound = "not-found"
	// CheckNotExecutable covers a file without the execute bit and a directory
	// — both are "found, cannot be run".
	CheckNotExecutable = "not-executable"
	// CheckHomeShortcut: a leading ~ is a shell convenience. lich spawns the
	// binary directly (internal/terminal/pty_unix.go), so nothing expands it and
	// the path is taken literally.
	CheckHomeShortcut = "home-shortcut"
	// CheckRelative: a relative path is resolved against each session's own
	// working directory, so the same setting means a different binary — or none
	// — per session. Reported rather than resolved: there is no one directory
	// this check could answer for.
	CheckRelative = "relative"
)

// Check is what a configured binary resolves to. Path is the absolute
// executable a session would spawn, empty for every status but CheckOK.
type Check struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// Verify resolves bin exactly as the spawn does — a bare name through $PATH, a
// path taken as written — and reports whether it can be run. It is what turns
// the binary setting from a field that accepts anything into one that answers.
func (s *Service) Verify(bin string) Check {
	bin = strings.TrimSpace(bin)
	switch {
	case bin == "":
		return Check{Status: CheckEmpty}
	case strings.HasPrefix(bin, "~"):
		return Check{Status: CheckHomeShortcut}
	case isRelativePath(bin):
		return Check{Status: CheckRelative}
	}
	path, err := s.lookPath(bin)
	if err == nil {
		return Check{Path: path, Status: CheckOK}
	}
	// A directory and a file without the execute bit both surface as a
	// permission error from LookPath's own executability test.
	if errors.Is(err, fs.ErrPermission) {
		return Check{Status: CheckNotExecutable}
	}
	return Check{Status: CheckNotFound}
}

// isRelativePath reports a value that names a location rather than a command,
// without naming it from the root. A bare "claude" is not one of these: it is a
// $PATH lookup, which resolves the same for every session.
func isRelativePath(bin string) bool {
	if filepath.IsAbs(bin) {
		return false
	}
	return strings.ContainsRune(bin, '/') || strings.ContainsRune(bin, filepath.Separator)
}
