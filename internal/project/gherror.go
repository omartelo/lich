package project

import (
	"context"
	"errors"
	"log/slog"
)

// ghFailure turns one failed gh call into the message the screen shows. See
// toolerror.go for why the raw stderr never travels with it.
func ghFailure(args []string, stderr string, runErr, ctxErr error) error {
	logToolFailure("gh", args, stderr, runErr)
	return errors.New(ghMessage(stderr, runErr, ctxErr))
}

// errUnreadableAnswer covers gh answering with something this build cannot
// decode — a field it renamed, a broken pipe mid-JSON. The decoder's own text
// names Go types, so it goes to the log like the rest.
//
// The messages here are sentences on purpose (capitalised, punctuated): they
// are read by a person on screen, not appended to another Go error.
func errUnreadableAnswer(err error) error {
	slog.Warn("gh answer could not be decoded", "err", err)
	return errors.New("GitHub's answer could not be read.")
}

// ghFailures maps what gh reports onto what the person reading the screen can
// do about it.
var ghFailures = []toolFailure{
	{
		"could not resolve to a repository",
		"GitHub does not show this repository to the account lich is using here. " +
			"Choose the account that can see it in Settings → Version Control.",
	},
	{"could not resolve to a pullrequest", "GitHub has no pull request with that number."},
	{
		"can not approve your own pull request",
		"GitHub does not let an account approve its own pull request — this one was opened by the " +
			"account lich is using here.",
	},
	{
		"must be part of the diff",
		"GitHub only takes a comment on a line this pull request actually changed. " +
			"Pick a line inside the diff.",
	},
	{
		"one pending review per pull request",
		"GitHub still holds an unsubmitted review from this account on this pull request. " +
			"Finish or discard it on github.com, then submit again.",
	},
	{
		"pull request review thread",
		"GitHub would not take that reply — the thread it answers is gone or already locked.",
	},
	{"gh auth login", "gh is not signed in to GitHub. Run `gh auth login` in a terminal."},
	{"not logged into", "gh is not signed in to GitHub. Run `gh auth login` in a terminal."},
	{"api rate limit exceeded", "GitHub is rate-limiting this account. Try again in a few minutes."},
	{"none of the git remotes", "This checkout's git remotes do not point at GitHub."},
	{"no git remotes found", "This checkout has no git remote to look up on GitHub."},
	{"not a git repository", "This folder is not a git repository."},
	// gh refuses a merge before it calls GitHub, and its one sentence carries
	// the reason in the tail: these two match that tail, because "not mergeable"
	// alone cannot tell a conflict from a rule and named the wrong one half the
	// time. The bare match stays last for a wording neither one catches.
	{
		"merge commit cannot be cleanly created",
		"This pull request conflicts with its base branch — merge the base in and resolve them first.",
	},
	{
		"base branch policy prohibits the merge",
		"A rule on the base branch blocks this merge. Its ruleset on GitHub says which one.",
	},
	{
		"rule violations found",
		"A ruleset on the base branch refused this merge — GitHub named the rules it broke; lich's log has them.",
	},
	{"not mergeable", "GitHub will not merge this pull request yet — it has conflicts or an unmet requirement."},
	{"protected branch", "A branch protection rule on GitHub blocks this merge."},
	{"required status check", "GitHub requires a status check that has not passed yet."},
	{"review required", "GitHub requires a review before this pull request can merge."},
}

// ghMessage picks the message for one gh failure. Anything unrecognised — a new
// gh wording, a forge error lich has never hit — falls back to a plain sentence
// that points at the log rather than pasting gh's own words on screen.
func ghMessage(stderr string, runErr, ctxErr error) string {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "GitHub did not answer in time. Check the connection and try again."
	}
	if missingTool(stderr, runErr) {
		return "The GitHub CLI (gh) is not installed."
	}
	if message, ok := matchFailure(ghFailures, stderr); ok {
		return message
	}
	return "gh could not complete the request. lich's log has what it reported."
}
