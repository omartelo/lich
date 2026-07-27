package project

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// TestGHMessage pins the contract the pull request screen depends on: every
// failure comes back as a sentence about what happened, and gh's own words are
// never part of it.
func TestGHMessage(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		runErr error
		ctxErr error
		want   string
	}{
		{
			"a repository the account cannot see points at the account picker",
			"GraphQL: Could not resolve to a Repository with the name 'acme/private'. (repository)",
			errTest, nil,
			"GitHub does not show this repository to the account lich is using here. " +
				"Choose the account that can see it in Settings → Version Control.",
		},
		{
			"a missing pull request is not the same failure",
			"GraphQL: Could not resolve to a PullRequest with the number 9999.",
			errTest, nil,
			"GitHub has no pull request with that number.",
		},
		{
			"logged out",
			"To get started with GitHub CLI, please run: gh auth login",
			errTest, nil,
			"gh is not signed in to GitHub. Run `gh auth login` in a terminal.",
		},
		{
			"rate limited",
			"HTTP 403: API rate limit exceeded for user ID 42.",
			errTest, nil,
			"GitHub is rate-limiting this account. Try again in a few minutes.",
		},
		{
			"no GitHub remote",
			"none of the git remotes configured for this repository point to a known GitHub host",
			errTest, nil,
			"This checkout's git remotes do not point at GitHub.",
		},
		{
			"an unmergeable pull request",
			"Pull request is not mergeable: the merge commit cannot be cleanly created.",
			errTest, nil,
			"GitHub will not merge this pull request yet — it has conflicts or an unmet requirement.",
		},
		{
			"gh missing, reported by exec",
			"", &exec.Error{Name: "gh", Err: exec.ErrNotFound}, nil,
			"The GitHub CLI (gh) is not installed.",
		},
		{
			"a timeout beats whatever the killed process wrote",
			"signal: killed", errTest, context.DeadlineExceeded,
			"GitHub did not answer in time. Check the connection and try again.",
		},
		{
			"an unknown failure never quotes gh",
			"GraphQL: something lich has never seen (field)",
			errTest, nil,
			"gh could not complete the request. lich's log has what it reported.",
		},
		{
			"an empty stderr still says something",
			"", errTest, nil,
			"gh could not complete the request. lich's log has what it reported.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ghMessage(tt.stderr, tt.runErr, tt.ctxErr)
			if got != tt.want {
				t.Errorf("ghMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGHMessageLeaksNothing is the rule itself rather than one case of it: no
// message may carry gh's vocabulary onto the screen.
func TestGHMessageLeaksNothing(t *testing.T) {
	leaks := []string{"graphql", "stderr", "exit status", "http 4", "http 5", "gh: "}
	stderrs := []string{
		"GraphQL: Could not resolve to a Repository with the name 'acme/private'. (repository)",
		"HTTP 403: API rate limit exceeded for user ID 42.",
		"gh: command not found",
		"exit status 1",
		"",
	}
	for _, stderr := range stderrs {
		got := strings.ToLower(ghMessage(stderr, errTest, nil))
		for _, leak := range leaks {
			if strings.Contains(got, leak) {
				t.Errorf("ghMessage(%q) = %q, which leaks %q", stderr, got, leak)
			}
		}
	}
}

func TestErrUnreadableAnswer(t *testing.T) {
	err := errUnreadableAnswer(errors.New("invalid character 'n' looking for beginning of value"))
	if want := "GitHub's answer could not be read."; err.Error() != want {
		t.Errorf("errUnreadableAnswer() = %q, want %q", err, want)
	}
}
