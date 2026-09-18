package project

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// CarryUncommitted copies the work src has not committed onto dst, so a fork
// starts from the checkout as it stands rather than from its last commit — the
// moment a fork is worth making is usually the moment there is uncommitted work
// to take two ways.
//
// Both checkouts must sit on the same commit, which is what the caller picking
// src's own branch as the base guarantees: the patch is read against src's HEAD
// and applied to dst's, and git refuses it outright if they differ.
//
// Everything lands unstaged in dst. Which of src's changes were staged is not
// carried: it is a fact about the commit src is preparing, and dst is a
// different piece of work from here on.
//
// What git ignores is not carried either (--exclude-standard), and it is the
// one part of the checkout a fork does not get from the session it forked:
// seedWorktree copies the ignored files the project names (`.env*` by default)
// out of the *main* checkout into every new worktree, and the setup script
// builds the rest. So an ignored file the forked session edited arrives as the
// main checkout's copy of it.
func (s *Service) CarryUncommitted(src, dst string) error {
	// --binary, so an edited image or any other non-text file comes over as
	// bytes instead of as an unappliable "Binary files differ" line.
	patch, err := runGit(src, "diff", "HEAD", "--binary")
	if err != nil {
		return err
	}
	if strings.TrimSpace(patch) != "" {
		if err := applyPatch(dst, patch); err != nil {
			return err
		}
	}
	return copyUntracked(src, dst)
}

// copyUntracked copies src's untracked, non-ignored files into dst. They are
// invisible to `git diff`, and they are where a half-written new file lives —
// carrying the diff without them would hand the fork a checkout that does not
// build.
func copyUntracked(src, dst string) error {
	out, err := runGit(src, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		if err := carryFile(filepath.Join(src, rel), filepath.Join(dst, rel)); err != nil {
			return fmt.Errorf("Copying %s into the new worktree failed: %w", rel, err)
		}
	}
	return nil
}

// carryFile is seedWorktree's copyFile with the two answers this caller gives
// differently. A symlink or any other non-regular file is skipped rather than
// reported: git lists them and a fork missing one is a smaller surprise than a
// fork that turned it into a file. So is a file that is already gone — the
// listing and the copy are two calls, and a build running in the source
// checkout deletes its own scratch files between them. Every other failure is
// the caller's to report: silently dropping a file the user is about to look
// for is the outcome this carry exists to avoid.
func carryFile(from, to string) error {
	if err := copyFile(from, to); err != nil {
		if errors.Is(err, errNotRegular) || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}
