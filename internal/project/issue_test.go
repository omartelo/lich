package project

import (
	"errors"
	"strings"
	"testing"
)

func TestIssue(t *testing.T) {
	t.Run("decodes and asks gh for the number", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"number":381,"title":"Sandbox backend","body":"why","url":"https://github.com/o/l/issues/381"}`)}
		issue, err := withGH(gh).Issue("/repo", 381)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue.Number != 381 || issue.Title != "Sandbox backend" || issue.Body != "why" {
			t.Errorf("decoded %+v", issue)
		}
		want := []string{"issue", "view", "381", "--json", "number,title,body,url"}
		if strings.Join(gh.args, " ") != strings.Join(want, " ") {
			t.Errorf("gh args = %v, want %v", gh.args, want)
		}
		if gh.dir != "/repo" {
			t.Errorf("gh ran in %q, want /repo", gh.dir)
		}
	})

	t.Run("a pull request is refused, not answered", func(t *testing.T) {
		// GitHub numbers issues and pull requests in one sequence, so gh answers
		// `issue view` with a pull request. Taking it would branch off the base
		// under its title instead of checking out its head.
		gh := &fakeGH{out: []byte(`{"number":453,"title":"feat: open a project","url":"https://github.com/o/l/pull/453"}`)}
		_, err := withGH(gh).Issue("/repo", 453)
		if err == nil {
			t.Fatal("a pull request was accepted as an issue")
		}
		if !strings.Contains(err.Error(), "pull request") {
			t.Errorf("message does not say what it is: %v", err)
		}
	})

	t.Run("a number gh cannot be asked for is refused before the call", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).Issue("/repo", 0); err == nil {
			t.Fatal("zero was accepted as an issue number")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times for a number that is not one", gh.calls)
		}
	})

	t.Run("gh's failure travels", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("gh said no")}
		if _, err := withGH(gh).Issue("/repo", 7); err == nil {
			t.Fatal("a failed gh call answered an issue")
		}
	})

	t.Run("an answer that cannot be decoded is not an issue", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"number":`)}
		if _, err := withGH(gh).Issue("/repo", 7); err == nil {
			t.Fatal("half a JSON object decoded")
		}
	})

	t.Run("a long body is cut", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"number":7,"url":"https://github.com/o/l/issues/7","body":"` +
			strings.Repeat("a", maxIssueBody+50) + `"}`)}
		issue, err := withGH(gh).Issue("/repo", 7)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(issue.Body, "[…]") {
			t.Errorf("a cut body does not say it was cut: %q", issue.Body[len(issue.Body)-10:])
		}
		if len([]rune(issue.Body)) <= maxIssueBody {
			t.Errorf("body is %d runes, want the cap plus its marker", len([]rune(issue.Body)))
		}
	})
}

func TestTruncateRunes(t *testing.T) {
	// The cap is pinned as a literal and crossed by one, rather than derived
	// from the constant: a test that follows the constant proves nothing about
	// where the boundary is.
	const limit = 5
	tests := []struct {
		name, text, want string
	}{
		{"under the cap", "abcd", "abcd"},
		{"exactly the cap", "abcde", "abcde"},
		{"one over", "abcdef", "abcde\n\n[…]"},
		{"empty", "", ""},
		// Multi-byte runes: cutting at 5 bytes would land inside a codepoint.
		{"runes, not bytes", "áéíóúx", "áéíóú\n\n[…]"},
		{"trailing space before the cut is dropped", "abcd ef", "abcd\n\n[…]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateRunes(tt.text, limit); got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.text, limit, got, tt.want)
			}
		})
	}
}

// TestIssueNotFoundMessage pins the gh wording an unknown number produces onto
// the sentence the dialog shows. The literal is what gh answered in the field —
// a reword there is a fallback message on screen, never a wrong one.
func TestIssueNotFoundMessage(t *testing.T) {
	const stderr = "GraphQL: Could not resolve to an issue or pull request with the number of 99999. (repository.issue)"
	got := ghMessage(stderr, errors.New("exit status 1"), nil)
	if got != "GitHub has no issue with that number." {
		t.Errorf("ghMessage = %q", got)
	}
}
