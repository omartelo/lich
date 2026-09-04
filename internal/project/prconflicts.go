package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// prConflictBudget caps the fetch behind the conflicting-file list. Both commits
// have to be here before anything can be merged, and that fetch is the one part
// of this answer that reaches the network.
const prConflictBudget = 60 * time.Second

// PullRequestConflicts lists the files a merge of pull request number into base
// would collide on. GitHub publishes that a pull request conflicts and never
// where: its answer is one word (mergeable: CONFLICTING), which left the reader
// opening github.com or a session to find out which files it meant. This
// computes it instead — both commits are fetched into refs lich owns and merged
// in the object database alone (mergeConflicts), never in a worktree, an index
// or HEAD.
//
// nil means the merge is clean, so this is asked only once GitHub has said it
// is not: the fetch is a network round trip, not something to poll.
//
// origin is the remote and refs/pull/<n>/head is GitHub's own ref — a fork's
// pull request included, since GitHub keeps that ref on the base repository.
func (s *Service) PullRequestConflicts(path string, number int, base string) ([]string, error) {
	if number <= 0 || base == "" {
		return nil, errors.New("Listing the conflicting files needs a pull request number and its base branch.")
	}
	head := fmt.Sprintf("refs/lich/pr/%d/head", number)
	baseRef := fmt.Sprintf("refs/lich/pr/%d/base", number)
	if err := fetchPRPair(path, number, base, head, baseRef); err != nil {
		return nil, err
	}
	return mergeConflicts(path, baseRef, head), nil
}

// fetchPRPair brings the pull request's head and the tip of its base branch into
// lich's own refs, in one round trip.
//
// Named destinations rather than FETCH_HEAD: that file is shared by every
// worktree of the repository and the base-status fetch (basestatus.go) writes it
// from a background goroutine, so two answers would mix into one. They are
// forced because the refs are scratch space holding the previous answer, which
// the new tip is routinely not a fast-forward of.
func fetchPRPair(path string, number int, base, head, baseRef string) error {
	ctx, cancel := context.WithTimeout(context.Background(), prConflictBudget)
	defer cancel()
	args := []string{"-C", path, "fetch", "--quiet", "--no-tags", "--force", "origin",
		fmt.Sprintf("refs/pull/%d/head:%s", number, head),
		"refs/heads/" + base + ":" + baseRef,
	}
	cmd := commandContext(ctx, "git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logToolFailure("git", args, stderr.String(), err)
		if ctx.Err() != nil {
			return fmt.Errorf("Reading pull request #%d took longer than %s and was given up on.", number, prConflictBudget)
		}
		return errors.New(gitMessage(stderr.String(), err))
	}
	return nil
}
