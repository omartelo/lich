package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// emptyTreeHash is git's well-known empty tree object, the diff base for a
// repository whose HEAD does not exist yet (no commits).
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// maxUntrackedDiffSize caps how large an untracked file may be before it is
// skipped in the textual diff, matching countFileLines' ceiling.
const maxUntrackedDiffSize = 10 << 20

// DiffText returns the full unified diff of uncommitted changes (staged +
// unstaged + untracked) against HEAD. A repository without commits diffs
// against git's empty tree. Untracked files are rendered as new-file hunks so
// the review panel shows them alongside tracked changes.
func (s *Service) DiffText(path string) (string, error) {
	base := "HEAD"
	if _, err := runGit(path, "rev-parse", "--verify", "HEAD"); err != nil {
		base = emptyTreeHash
	}
	tracked, err := runGit(path, "diff", base)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.WriteString(tracked)
	for _, rel := range untrackedFiles(path) {
		out.WriteString(untrackedDiff(path, rel))
	}
	return out.String(), nil
}

// untrackedFiles lists paths unknown to git, relative to the work tree root.
// Errors yield an empty list — the tracked diff is still worth returning.
func untrackedFiles(path string) []string {
	out, err := command("git", "-C", path, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil
	}
	var files []string
	for rel := range strings.SplitSeq(string(out), "\x00") {
		if rel != "" {
			files = append(files, rel)
		}
	}
	return files
}

// untrackedDiff renders an untracked file as a new-file unified diff via
// git diff --no-index, which exits 1 when the files differ — success here.
// Any other failure (file vanished between ls-files and now) yields "".
func untrackedDiff(dir, rel string) string {
	if info, err := os.Stat(filepath.Join(dir, rel)); err != nil || !info.Mode().IsRegular() || info.Size() > maxUntrackedDiffSize {
		return ""
	}
	cmd := command("git", "-C", dir, "diff", "--no-index", "--", os.DevNull, rel)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
		return ""
	}
	return string(out)
}
