package project

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// An issue read is one round-trip like a pull request read, and shares its
// budget; the alias only spells the call site honestly.
const issueReadTimeout = prReadTimeout

// maxIssueBody caps the issue text handed to a session. A body is prose, but a
// template-heavy one carries logs and stack traces whole, and this is pasted
// into a terminal prompt — past a few pages it is scrollback, not context. The
// issue's URL travels with it, so the cut costs nothing but a click.
const maxIssueBody = 4000

// Issue is the little of a GitHub issue the New worktree dialog needs: what to
// name the branch after, and what to tell the session it opens.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

// Issue resolves one GitHub issue through gh, as the account the project
// selected. The title names the branch the dialog is about to create, and the
// body is what that worktree's session finds at its prompt.
//
// A pull request is refused rather than answered. GitHub numbers issues and
// pull requests in one sequence and its API calls them all issues, so
// `gh issue view <n>` answers for either — and taking the pull request would
// branch off the base under its title instead of checking its head branch out,
// which is a different flow entirely (CreateWorktreeFromPR). The URL is the
// only field in the answer that tells the two apart.
func (s *Service) Issue(path string, number int) (Issue, error) {
	if number <= 0 {
		return Issue{}, fmt.Errorf("%d is not an issue number.", number)
	}
	// The selector is an int, so it can never reach gh's argument slot as a
	// "-"-prefixed value that would parse as a flag — prSelector's reasoning.
	out, err := s.gh(issueReadTimeout, path,
		"issue", "view", strconv.Itoa(number), "--json", "number,title,body,url")
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, errUnreadableAnswer(err)
	}
	if strings.Contains(issue.URL, "/pull/") {
		return Issue{}, fmt.Errorf("#%d is a pull request, not an issue — open it from the Pulls screen.", number)
	}
	issue.Body = truncateRunes(issue.Body, maxIssueBody)
	return issue, nil
}

// truncateRunes cuts text to at most limit runes, marking the cut so whoever
// reads it knows there is more. Runes rather than bytes: a cut through a
// codepoint is a mojibake tail, and issue bodies are not ASCII.
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimRight(string(runes[:limit]), " \t\n") + "\n\n[…]"
}
