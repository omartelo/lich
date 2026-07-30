package project

import (
	"slices"
	"testing"
)

func TestParseBranchRules(t *testing.T) {
	t.Run("a ruleset that narrows the merge methods", func(t *testing.T) {
		out := []byte(`[
			{"type":"deletion","parameters":null},
			{"type":"commit_message_pattern","parameters":{"pattern":"^(feat|fix): .+"}},
			{"type":"pull_request","parameters":{"allowed_merge_methods":["squash"],"required_approving_review_count":1}}
		]`)
		rules := parseBranchRules(out)
		if want := []string{"squash"}; !slices.Equal(rules.AllowedMergeMethods, want) {
			t.Errorf("methods = %v, want %v", rules.AllowedMergeMethods, want)
		}
	})

	t.Run("a branch nothing governs leaves every method standing", func(t *testing.T) {
		if got := parseBranchRules([]byte(`[]`)).AllowedMergeMethods; len(got) != 0 {
			t.Errorf("methods = %v, want none named", got)
		}
	})

	t.Run("a ruleset with rules but nothing about merging", func(t *testing.T) {
		out := []byte(`[{"type":"pull_request","parameters":{"required_approving_review_count":2}}]`)
		if got := parseBranchRules(out).AllowedMergeMethods; len(got) != 0 {
			t.Errorf("methods = %v, want none named", got)
		}
	})

	// The menu must never end up narrower than the truth because an answer was
	// unreadable — a repository GitHub reports on differently, or an error body.
	t.Run("an answer this cannot read names no method at all", func(t *testing.T) {
		for _, body := range []string{`{"message":"Not Found"}`, `not json`, ``} {
			if got := parseBranchRules([]byte(body)).AllowedMergeMethods; len(got) != 0 {
				t.Errorf("body %q gave methods %v, want none", body, got)
			}
		}
	})
}

func TestBranchRulesFlow(t *testing.T) {
	t.Run("the branch is addressed in the path, escaped", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`[]`)}
		if _, err := withGH(gh).BranchRules("/repo", "feat/IMPEX-728"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"api", "repos/{owner}/{repo}/rules/branches/feat%2FIMPEX-728"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("no branch never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).BranchRules("/repo", ""); err == nil {
			t.Error("expected an error without a branch")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("a gh failure reaches the caller as written", func(t *testing.T) {
		gh := &fakeGH{err: testError("GitHub is rate-limiting this account. Try again in a few minutes.")}
		if _, err := withGH(gh).BranchRules("/repo", "main"); err == nil {
			t.Error("expected the runner's error")
		}
	})
}
