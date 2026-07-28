package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/relpath"
)

// DiscardFile reverts one file's uncommitted changes. A file known to HEAD is
// checked out from it (restoring index and worktree in one step); a new file
// (staged or untracked) is unstaged and deleted from disk. rel must be a
// repo-relative path — the review panel passes paths parsed from git's own
// diff output.
func (s *Service) DiscardFile(path, rel string) error {
	if err := relpath.Validate(rel); err != nil {
		return err
	}
	// Quiet: "HEAD does not know this file" is the answer that routes to the
	// new-file branch below, not a failure worth a log line per discard.
	if _, known := gitQuiet(path, "cat-file", "-e", "HEAD:"+rel); known {
		_, err := runGit(path, "checkout", "HEAD", "--", rel)
		return err
	}
	if _, err := runGit(path, "rm", "-f", "--cached", "--ignore-unmatch", "--", rel); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(path, rel)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", rel, err)
	}
	return nil
}
