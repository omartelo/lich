// Package relpath validates the repo-relative paths lich receives from the
// page — a file picked in the review panel, a path parsed out of git's diff
// output — before they are joined onto a work-tree root. It is the one copy of
// that rule: every surface that opens, reads or deletes a file by relative path
// answers to it, so an escape closed here is closed everywhere.
//
// Two rules, because the callers do not all want the same one. Validate is the
// lexical rule and answers for the path as written — what a caller passing rel
// to git or unlinking it needs. Resolve adds the filesystem: it follows the
// links and proves the result is still in the tree, which only a caller that
// reads the file's bytes has to care about.
package relpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate rejects rooted paths, parent-directory escapes and the work-tree
// root itself, so joining rel onto a work-tree root can never leave it — and
// can never name the whole tree. The root is refused because a caller that
// means one file is handed every file instead: `git rm -f --cached -- .`
// empties the index (see project.DiscardFile). It is also what an empty path
// cleans to, so a missing argument stops here rather than at the git call.
func Validate(rel string) error {
	clean := filepath.Clean(rel)
	if clean == "." || isRooted(clean) || Escapes(clean) {
		return fmt.Errorf("invalid repository path %q", rel)
	}
	return nil
}

// Resolve validates rel, joins it onto root and returns the real path to open:
// every symlink resolved, and proven to still be inside root. Validate alone
// answers the path as written, which is the whole rule for a caller that hands
// rel to git or unlinks it — but not for one that reads bytes, because a
// checkout may ship a link and following it lands wherever the link points.
//
// git stores such an entry as its target text, so the diff beside a preview
// says `../../etc/hosts` while the preview would print that file. Refusing here
// is what keeps the two telling the same story about the same tree.
//
// Both sides are resolved before they are compared: a checkout is routinely
// reached through a link itself (macOS `/var` is `/private/var`), and measuring
// a resolved target against an unresolved root calls every file an escape.
func Resolve(root, rel string) (string, error) {
	if err := Validate(rel); err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve checkout %s: %w", root, err)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", rel, err)
	}
	inside, err := filepath.Rel(realRoot, full)
	if err != nil || Escapes(inside) {
		return "", fmt.Errorf("%q leaves the checkout", rel)
	}
	return full, nil
}

// Escapes reports whether a relative path names the parent or anything below
// it. Callers use filepath.Rel first when they begin with absolute paths.
func Escapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isRooted reports whether a cleaned path starts at a root. IsAbs alone is not
// enough on Windows, where it demands a volume name and so reads "/etc/passwd"
// as relative; a work-tree path is always relative, so a leading separator is
// refused too (os.IsPathSeparator counts "\" only where it is one).
func isRooted(clean string) bool {
	return filepath.IsAbs(clean) || os.IsPathSeparator(clean[0])
}
