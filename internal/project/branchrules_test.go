package project

import (
	"slices"
	"testing"
)

func TestParseBranchRules(t *testing.T) {
	t.Run("a ruleset that narrows the merge methods", func(t *testing.T) {
		out := []byte(`[
			{"type":"deletion","parameters":null},
			{"type":"commit_message_pattern","parameters":{"operator":"regex","pattern":"^(feat|fix): .+"}},
			{"type":"pull_request","parameters":{"allowed_merge_methods":["squash"],"required_approving_review_count":1}}
		]`)
		rules := parseBranchRules(out)
		if want := []string{"squash"}; !slices.Equal(rules.AllowedMergeMethods, want) {
			t.Errorf("methods = %v, want %v", rules.AllowedMergeMethods, want)
		}
	})

	// The rule that sends an approved, green, conflict-free pull request to
	// BLOCKED. Reading it is the whole reason the screen can name a cause.
	t.Run("commit pattern rules are kept with what they test", func(t *testing.T) {
		out := []byte(`[
			{"type":"commit_message_pattern","parameters":{"operator":"regex","pattern":"^(feat|fix): .+","negate":false,"name":"Conventional Commits"}},
			{"type":"committer_email_pattern","parameters":{"operator":"ends_with","pattern":"@example.com","negate":true}}
		]`)
		got := parseBranchRules(out).CommitPatterns
		want := []CommitPattern{
			{Target: "message", Operator: "regex", Pattern: "^(feat|fix): .+"},
			{Target: "committer email", Operator: "ends_with", Pattern: "@example.com", Negate: true},
		}
		if !slices.Equal(got, want) {
			t.Errorf("patterns = %+v, want %+v", got, want)
		}
	})

	// A branch name pattern governs pushes, not merges: naming it beside the
	// rules that do block would send the reader after the wrong rule.
	t.Run("rules that gate a push rather than a merge are left out", func(t *testing.T) {
		out := []byte(`[
			{"type":"branch_name_pattern","parameters":{"operator":"starts_with","pattern":"feat/"}},
			{"type":"tag_name_pattern","parameters":{"operator":"starts_with","pattern":"v"}},
			{"type":"commit_message_pattern","parameters":{"operator":"contains","pattern":""}}
		]`)
		if got := parseBranchRules(out).CommitPatterns; len(got) != 0 {
			t.Errorf("patterns = %+v, want none", got)
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
		if len(gh.history) == 0 || !slices.Equal(gh.history[0], want) {
			t.Errorf("args = %v, want %v", gh.history, want)
		}
	})

	t.Run("administering the repository is what opens the bypass", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{[]byte(`[]`), []byte(`{"viewerPermission":"ADMIN"}`)}}
		rules, err := withGH(gh).BranchRules("/repo", "develop")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rules.ViewerCanBypass {
			t.Error("an administering account should be offered the bypass")
		}
		want := []string{"repo", "view", "--json", "viewerPermission"}
		if len(gh.history) != 2 || !slices.Equal(gh.history[1], want) {
			t.Errorf("second call = %v, want %v", gh.history, want)
		}
	})

	// The bypass is the one answer here that fails closed: offered to an account
	// that does not have it, it reads as a broken button rather than a
	// permission it never held.
	t.Run("anything short of an answer keeps the bypass hidden", func(t *testing.T) {
		for _, second := range []string{
			`{"viewerPermission":"WRITE"}`,
			`{"viewerPermission":"MAINTAIN"}`,
			`{}`,
			`not json`,
			``,
		} {
			gh := &fakeGH{outs: [][]byte{[]byte(`[]`), []byte(second)}}
			rules, err := withGH(gh).BranchRules("/repo", "develop")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rules.ViewerCanBypass {
				t.Errorf("permission answer %q offered the bypass", second)
			}
		}
	})

	// The rules are worth drawing even when the permission read fails, so the
	// failure hides the bypass and nothing else.
	t.Run("a failed permission read costs the bypass, not the rules", func(t *testing.T) {
		gh := &fakeGH{
			outs: [][]byte{[]byte(`[{"type":"pull_request","parameters":{"allowed_merge_methods":["squash"]}}]`)},
			errs: []error{nil, testError("gh could not complete the request.")},
		}
		rules, err := withGH(gh).BranchRules("/repo", "develop")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rules.ViewerCanBypass {
			t.Error("a failed permission read must not offer the bypass")
		}
		if want := []string{"squash"}; !slices.Equal(rules.AllowedMergeMethods, want) {
			t.Errorf("methods = %v, want %v", rules.AllowedMergeMethods, want)
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
