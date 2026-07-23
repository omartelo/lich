package project

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// defaultIncludes seeds every new worktree when the repository ships no
// .worktreeinclude: env files by basename, at any depth.
var defaultIncludes = []string{".env*"}

// includeFile is the repository-root file that overrides defaultIncludes: one
// glob per line, # comments and blank lines skipped. A present file governs
// alone — an empty one seeds nothing.
const includeFile = ".worktreeinclude"

// seedWorktree copies gitignored files matching the include patterns from the
// main checkout into a fresh worktree. A worktree starts with tracked files
// only, so without this every new checkout is missing its .env files and
// local credentials. Failures never fail the creation — a worktree without
// its env files is still usable — they are logged and the rest is copied.
// Returns the repo-relative paths copied, for the log and the tests.
func seedWorktree(projectPath, wtPath string) []string {
	patterns := includePatterns(projectPath)
	if len(patterns) == 0 {
		return nil
	}
	// --directory collapses wholly-ignored directories (node_modules) into one
	// entry instead of enumerating every file inside; -z spares us git's
	// C-style quoting of non-ASCII names.
	out, err := runGit(projectPath, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		slog.Warn("worktree seed: list ignored files", "err", err)
		return nil
	}
	var copied []string
	for entry := range strings.SplitSeq(out, "\x00") {
		// Directory entries carry a trailing slash: whole ignored trees are
		// deliberately not copied (that is dependency/build output territory —
		// the setup script's job, not the seed's).
		if entry == "" || strings.HasSuffix(entry, "/") {
			continue
		}
		if !matchesInclude(entry, patterns) {
			continue
		}
		src := filepath.Join(projectPath, filepath.FromSlash(entry))
		dst := filepath.Join(wtPath, filepath.FromSlash(entry))
		if err := copyFile(src, dst); err != nil {
			slog.Warn("worktree seed: copy", "file", entry, "err", err)
			continue
		}
		copied = append(copied, entry)
	}
	if len(copied) > 0 {
		slog.Info("worktree seed", "worktree", wtPath, "files", copied)
	}
	return copied
}

// includePatterns returns the seed patterns: .worktreeinclude at the repo
// root when present, defaultIncludes otherwise.
func includePatterns(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, includeFile))
	if err != nil {
		return defaultIncludes
	}
	patterns := []string{}
	for _, line := range splitLines(string(data)) {
		if !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}

// matchesInclude reports whether a repo-relative path (slash form) matches any
// pattern. The gitignore convention, reduced to path.Match globs: a pattern
// without a slash matches the basename at any depth, one with a slash matches
// the whole relative path. No "**" and no "!" negation.
func matchesInclude(rel string, patterns []string) bool {
	for _, p := range patterns {
		target := rel
		if !strings.Contains(p, "/") {
			target = path.Base(rel)
		}
		if ok, err := path.Match(p, target); ok && err == nil {
			return true
		}
	}
	return false
}

// copyFile copies a regular file preserving its permission bits — a private
// key seeded world-readable would be a downgrade. Symlinks and other
// non-regular files are refused so the caller logs them.
func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}
