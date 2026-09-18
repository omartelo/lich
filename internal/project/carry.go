package project

import (
	"fmt"
	"io"
	"os"
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
// What git ignores is not carried either (--exclude-standard). A new worktree
// never had node_modules or a .env, and the project's setup script is what
// answers for those (seedWorktree).
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
		if err := copyInto(filepath.Join(src, rel), filepath.Join(dst, rel)); err != nil {
			return fmt.Errorf("Copying %s into the new worktree failed: %w", rel, err)
		}
	}
	return nil
}

// copyInto copies one regular file, creating the directories leading to it.
// Anything else git listed — a symlink, a socket a tool left behind — is
// skipped rather than dereferenced: a fork missing one is a smaller surprise
// than a fork that turned it into a file, and a dangling one would fail the
// whole carry.
func copyInto(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
