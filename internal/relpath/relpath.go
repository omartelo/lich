// Package relpath validates the repo-relative paths lich receives from the
// page — a file picked in the review panel, a path parsed out of git's diff
// output — before they are joined onto a work-tree root. It is the one copy of
// that rule: every surface that opens, reads or deletes a file by relative path
// answers to it, so an escape closed here is closed everywhere.
package relpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate rejects rooted paths and parent-directory escapes, so joining rel
// onto a work-tree root can never leave it.
func Validate(rel string) error {
	clean := filepath.Clean(rel)
	if isRooted(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid repository path %q", rel)
	}
	return nil
}

// isRooted reports whether a cleaned path starts at a root. IsAbs alone is not
// enough on Windows, where it demands a volume name and so reads "/etc/passwd"
// as relative; a work-tree path is always relative, so a leading separator is
// refused too (os.IsPathSeparator counts "\" only where it is one).
func isRooted(clean string) bool {
	return filepath.IsAbs(clean) || os.IsPathSeparator(clean[0])
}
