package project

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
// prURL is the pull request's own URL, which names the repository it lives on —
// see prRemote. refs/pull/<n>/head is GitHub's own ref, a fork's pull request
// included, since GitHub keeps that ref on the base repository.
func (s *Service) PullRequestConflicts(path string, number int, base, prURL string) ([]string, error) {
	if number <= 0 || base == "" {
		return nil, errors.New("Listing the conflicting files needs a pull request number and its base branch.")
	}
	head := fmt.Sprintf("refs/lich/pr/%d/head", number)
	baseRef := fmt.Sprintf("refs/lich/pr/%d/base", number)
	if err := fetchPRPair(path, number, base, head, baseRef, prRemote(path, prURL)); err != nil {
		return nil, err
	}
	files, ok := mergeConflicts(path, baseRef, head)
	if !ok {
		return nil, fmt.Errorf("This git could not merge pull request #%d to see where it collides; naming the files needs git 2.38 or newer.", number)
	}
	return files, nil
}

// prRemote names the remote holding the pull request. Not origin by assumption:
// in a clone of a fork, origin is the fork and the pull request — refs/pull and
// the base branch both — lives on the repository it was opened against. Every
// other pull request flow rides gh's own resolution of that repository; this one
// reads it back off the pull request's URL, which the screen already has, rather
// than spending a second round trip asking gh again.
//
// Falls back to origin: that is what a clone with one remote has, and a URL
// nothing matches is better fetched from the remote that usually holds it than
// not fetched at all.
func prRemote(path, prURL string) string {
	repo := repoFromPRURL(prURL)
	if repo == "" {
		return "origin"
	}
	out, ok := gitQuiet(path, "remote", "-v")
	if !ok {
		return "origin"
	}
	for _, line := range splitLines(out) {
		name, remoteURL, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		if remoteRepo(remoteURL) == repo {
			return name
		}
	}
	return "origin"
}

// repoFromPRURL reads "owner/name" out of a pull request URL
// (https://host/owner/name/pull/7), lowercased so it compares against a remote
// the way GitHub itself treats the pair: case-insensitively.
func repoFromPRURL(prURL string) string {
	parsed, err := url.Parse(prURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[0] + "/" + parts[1])
}

// remoteRepo reads the same "owner/name" out of a remote's URL, in either
// spelling git accepts: git@host:owner/name.git and https://host/owner/name.
// The " (fetch)"/" (push)" tail `git remote -v` writes goes with it.
func remoteRepo(remoteURL string) string {
	fields := strings.Fields(remoteURL)
	if len(fields) == 0 {
		return ""
	}
	trimmed := strings.TrimSuffix(fields[0], ".git")
	trimmed = strings.ReplaceAll(trimmed, ":", "/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(parts[len(parts)-2] + "/" + parts[len(parts)-1])
}

// fetchPRPair brings the pull request's head and the tip of its base branch into
// lich's own refs, in one round trip.
//
// Named destinations rather than FETCH_HEAD: that file is shared by every
// worktree of the repository and the base-status fetch (basestatus.go) writes it
// from a background goroutine, so two answers would mix into one. They are
// forced because the refs are scratch space holding the previous answer, which
// the new tip is routinely not a fast-forward of.
func fetchPRPair(path string, number int, base, head, baseRef, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), prConflictBudget)
	defer cancel()
	args := []string{"-C", path, "fetch", "--quiet", "--no-tags", "--force", remote,
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
