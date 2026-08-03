package project

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParsePRDetail(t *testing.T) {
	t.Run("open PR with mixed checks", func(t *testing.T) {
		out := []byte(`{
			"number": 88,
			"url": "https://github.com/omartelo/lich/pull/88",
			"state": "OPEN",
			"title": "feat: view and merge pull requests",
			"body": "makes the badge actionable",
			"isDraft": false,
			"mergeable": "MERGEABLE",
			"mergeStateStatus": "UNSTABLE",
			"baseRefName": "main",
			"headRefName": "quiet-willow",
			"changedFiles": 6,
			"statusCheckRollup": [
				{"status": "COMPLETED", "conclusion": "SUCCESS"},
				{"state": "SUCCESS"}
			],
			"commits": [
				{"oid": "a4dbc1f", "messageHeadline": "feat: the panel", "messageBody": "why"}
			]
		}`)
		pr, err := parsePRDetail(out, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil {
			t.Fatal("expected a detail, got nil")
		}
		if pr.Number != 88 || pr.Title != "feat: view and merge pull requests" {
			t.Errorf("wrong header: %+v", pr)
		}
		if pr.Mergeable != "MERGEABLE" || pr.BaseRefName != "main" || pr.HeadRefName != "quiet-willow" {
			t.Errorf("wrong refs/mergeable: %+v", pr)
		}
		if pr.MergeStateStatus != "UNSTABLE" {
			t.Errorf("wrong merge state status: %q", pr.MergeStateStatus)
		}
		if pr.Checks.Passed != 2 || pr.Checks.Total != 2 || pr.Checks.Failed != 0 {
			t.Errorf("wrong checks: %+v", pr.Checks)
		}
		if pr.ChangedFiles != 6 {
			t.Errorf("wrong changed files: %d", pr.ChangedFiles)
		}
		if len(pr.Commits) != 1 || pr.Commits[0].Headline != "feat: the panel" {
			t.Errorf("wrong commits: %+v", pr.Commits)
		}
	})

	t.Run("draft flag survives", func(t *testing.T) {
		pr, err := parsePRDetail([]byte(`{"number":9,"state":"OPEN","isDraft":true}`), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil || !pr.IsDraft {
			t.Errorf("expected draft detail, got %+v", pr)
		}
	})

	// The pair the session button reads: a fork alone says nothing, the maintainer
	// permission is what decides whether it can be worked on.
	t.Run("a fork PR carries its maintainer-edit permission", func(t *testing.T) {
		raw := `{"number":131,"state":"OPEN","isCrossRepository":true,"maintainerCanModify":true}`
		pr, err := parsePRDetail([]byte(raw), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil || !pr.IsCrossRepository || !pr.MaintainerCanModify {
			t.Errorf("expected an editable fork detail, got %+v", pr)
		}
	})

	t.Run("non-open PR is hidden", func(t *testing.T) {
		pr, err := parsePRDetail([]byte(`{"number":88,"state":"MERGED","url":"x"}`), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr != nil {
			t.Errorf("merged PR should yield nil, got %+v", pr)
		}
	})

	t.Run("no PR object is hidden", func(t *testing.T) {
		pr, err := parsePRDetail([]byte(`{}`), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr != nil {
			t.Errorf("empty payload should yield nil, got %+v", pr)
		}
	})

	t.Run("malformed JSON errors", func(t *testing.T) {
		if _, err := parsePRDetail([]byte(`{not json`), true); err == nil {
			t.Error("expected a decode error")
		}
	})
}

func TestReduceChecks(t *testing.T) {
	tests := []struct {
		name  string
		items []checkItem
		want  ChecksRollup
	}{
		{"none", nil, ChecksRollup{}},
		{
			"completed check runs pass and fail",
			[]checkItem{
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
				{Status: "COMPLETED", Conclusion: "FAILURE"},
			},
			ChecksRollup{Passed: 1, Failed: 1, Total: 2},
		},
		{
			"neutral and skipped count as pass",
			[]checkItem{
				{Status: "COMPLETED", Conclusion: "NEUTRAL"},
				{Status: "COMPLETED", Conclusion: "SKIPPED"},
			},
			ChecksRollup{Passed: 2, Total: 2},
		},
		{
			"in-flight check run is pending",
			[]checkItem{{Status: "IN_PROGRESS"}, {Status: "QUEUED"}},
			ChecksRollup{Pending: 2, Total: 2},
		},
		{
			"legacy status contexts map by state",
			[]checkItem{
				{State: "SUCCESS"},
				{State: "FAILURE"},
				{State: "ERROR"},
				{State: "PENDING"},
			},
			ChecksRollup{Passed: 1, Failed: 2, Pending: 1, Total: 4},
		},
		{
			"mixed shapes sum together",
			[]checkItem{
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
				{State: "SUCCESS"},
				{Status: "IN_PROGRESS"},
			},
			ChecksRollup{Passed: 2, Pending: 1, Total: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reduceChecks(tt.items); got != tt.want {
				t.Errorf("reduceChecks() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToCheckItems(t *testing.T) {
	t.Run("no checks yields no list", func(t *testing.T) {
		if got := toCheckItems(nil); got != nil {
			t.Errorf("toCheckItems(nil) = %+v, want nil", got)
		}
	})

	t.Run("both gh shapes flatten to one row", func(t *testing.T) {
		got := toCheckItems([]checkItem{
			{
				Name: "test (ubuntu)", WorkflowName: "CI",
				Status: "COMPLETED", Conclusion: "SUCCESS",
				DetailsURL:  "https://github.com/o/l/runs/1",
				StartedAt:   "2026-07-25T10:00:00Z",
				CompletedAt: "2026-07-25T10:02:30Z",
			},
			{
				Context: "ci/legacy", Description: "the old bot",
				State: "SUCCESS", TargetURL: "https://ci.example/1",
			},
		})
		if len(got) != 2 {
			t.Fatalf("want 2 rows, got %+v", got)
		}
		run := got[0]
		if run.Name != "test (ubuntu)" || run.Description != "CI" || run.State != checkPassed {
			t.Errorf("check run flattened wrong: %+v", run)
		}
		if run.URL != "https://github.com/o/l/runs/1" || run.CompletedAt == "" {
			t.Errorf("check run lost its link or timing: %+v", run)
		}
		ctx := got[1]
		// A StatusContext names itself with context/description/targetUrl; those
		// have to land in the same three fields as a CheckRun's.
		if ctx.Name != "ci/legacy" || ctx.Description != "the old bot" || ctx.URL != "https://ci.example/1" {
			t.Errorf("status context flattened wrong: %+v", ctx)
		}
	})

	t.Run("worst state first, gh's order within a state", func(t *testing.T) {
		got := toCheckItems([]checkItem{
			{Name: "passed-1", Status: "COMPLETED", Conclusion: "SUCCESS"},
			{Name: "running", Status: "IN_PROGRESS"},
			{Name: "failed", Status: "COMPLETED", Conclusion: "FAILURE"},
			{Name: "passed-2", Status: "COMPLETED", Conclusion: "SKIPPED"},
		})
		var names []string
		for _, c := range got {
			names = append(names, c.Name)
		}
		want := []string{"failed", "running", "passed-1", "passed-2"}
		if !slices.Equal(names, want) {
			t.Errorf("order = %v, want %v", names, want)
		}
	})

	t.Run("the list agrees with the rollup", func(t *testing.T) {
		items := []checkItem{
			{Name: "a", Status: "COMPLETED", Conclusion: "SUCCESS"},
			{Name: "b", Status: "QUEUED"},
			{Name: "c", State: "ERROR"},
		}
		rollup := reduceChecks(items)
		var passed, failed, pending int
		for _, c := range toCheckItems(items) {
			switch c.State {
			case checkPassed:
				passed++
			case checkFailed:
				failed++
			case checkPending:
				pending++
			}
		}
		if passed != rollup.Passed || failed != rollup.Failed || pending != rollup.Pending {
			t.Errorf("list %d/%d/%d disagrees with rollup %+v", passed, failed, pending, rollup)
		}
	})
}

func TestToCommits(t *testing.T) {
	t.Run("no commits yields no list", func(t *testing.T) {
		if got := toCommits(nil); got != nil {
			t.Errorf("toCommits(nil) = %+v, want nil", got)
		}
	})

	t.Run("subject, body and author survive in gh's order", func(t *testing.T) {
		got := toCommits([]ghCommit{
			{
				OID:             "a4dbc1f",
				MessageHeadline: "fix: retire the badge",
				MessageBody:     "The footer badge only looked up again on HEAD moves.",
				CommittedDate:   "2026-07-25T22:32:47Z",
				Authors:         []ghCommitAuthor{{Login: "omartelo", Name: "omartelo"}},
			},
			{OID: "f55ebe9", MessageHeadline: "docs: changelog"},
		})
		if len(got) != 2 {
			t.Fatalf("want 2 commits, got %+v", got)
		}
		first := got[0]
		if first.OID != "a4dbc1f" || first.Headline != "fix: retire the badge" {
			t.Errorf("wrong header: %+v", first)
		}
		if !strings.Contains(first.Body, "HEAD moves") || first.Author != "omartelo" {
			t.Errorf("lost body or author: %+v", first)
		}
		if first.Date != "2026-07-25T22:32:47Z" {
			t.Errorf("wrong date: %q", first.Date)
		}
		// A one-line commit has no body and a web-flow merge commit no author;
		// both are rows, not errors.
		if got[1].Body != "" || got[1].Author != "" {
			t.Errorf("second commit should be bare: %+v", got[1])
		}
	})

	t.Run("a nameless login falls back to the name", func(t *testing.T) {
		got := toCommits([]ghCommit{{
			OID:     "c0ffee0",
			Authors: []ghCommitAuthor{{Name: "Ada Lovelace"}},
		}})
		if len(got) != 1 || got[0].Author != "Ada Lovelace" {
			t.Errorf("author fallback failed: %+v", got)
		}
	})
}

func TestMergeArgs(t *testing.T) {
	// An unrecognised method must be refused before any gh shell-out.
	if _, err := mergeArgs(0, "force", "", "", false); err == nil {
		t.Error("expected an error for an unknown merge method")
	}

	tests := []struct {
		name    string
		number  int
		method  string
		subject string
		body    string
		admin   bool
		want    []string
	}{
		{"quick squash", 0, "squash", "", "", false, []string{"pr", "merge", "--squash"}},
		{"quick merge", 0, "merge", "", "", false, []string{"pr", "merge", "--merge"}},
		{"quick rebase", 0, "rebase", "", "", false, []string{"pr", "merge", "--rebase"}},
		{
			"squash with edited message",
			0, "squash", "title (#1)", "details", false,
			[]string{"pr", "merge", "--squash", "--subject", "title (#1)", "--body", "details"},
		},
		{
			"empty body still passes both flags",
			0, "merge", "subject only", "", false,
			[]string{"pr", "merge", "--merge", "--subject", "subject only", "--body", ""},
		},
		{
			"rebase ignores an edited message",
			0, "rebase", "unused", "unused", false,
			[]string{"pr", "merge", "--rebase"},
		},
		// The selector sits ahead of the flags — gh reads a trailing number as
		// an argument to whichever flag precedes it.
		{
			"a numbered pull request is selected before the flags",
			42, "squash", "", "", false,
			[]string{"pr", "merge", "42", "--squash"},
		},
		{
			"the selector survives an edited message",
			42, "squash", "title (#42)", "",
			false,
			[]string{"pr", "merge", "42", "--squash", "--subject", "title (#42)", "--body", ""},
		},
		{
			"a bypass merge carries --admin",
			42, "squash", "", "", true,
			[]string{"pr", "merge", "42", "--squash", "--admin"},
		},
		{
			"a bypass merge keeps an edited message",
			0, "squash", "refactor(dashboard): fix", "", true,
			[]string{
				"pr", "merge", "--squash",
				"--subject", "refactor(dashboard): fix", "--body", "", "--admin",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergeArgs(tt.number, tt.method, tt.subject, tt.body, tt.admin)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("mergeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNoPullRequest(t *testing.T) {
	tests := []struct {
		stderr string
		want   bool
	}{
		{`no pull requests found for branch "quiet-willow"`, true},
		{"no pull requests found for current branch", true},
		{"", false},
		{"could not find any commits between main and X", false},
		{"gh: command not found", false},
	}
	for _, tt := range tests {
		if got := isNoPullRequest(tt.stderr); got != tt.want {
			t.Errorf("isNoPullRequest(%q) = %v, want %v", tt.stderr, got, tt.want)
		}
	}
}

// ghError's contract moved to gherror.go: gh's stderr no longer reaches the
// caller at all. TestGHMessage covers what replaced it.

// errTest is a fixed error for ghError's fallback branch.
var errTest = testError("sentinel")

type testError string

func (e testError) Error() string { return string(e) }

// fakeGH stands in for the gh CLI: it records the invocation and replays a
// canned result, so the pull request flows run without a GitHub remote.
//
// out and err answer every call. A flow that makes more than one gh call — and
// needs a different answer from each — sets outs/errs instead, one entry per
// call in order; a call past the end of either falls back to out/err. history
// keeps every invocation, while args stays the last one.
type fakeGH struct {
	out  []byte
	outs [][]byte
	err  error
	errs []error

	calls   int
	timeout time.Duration
	dir     string
	token   string
	args    []string
	history [][]string
}

func (f *fakeGH) run(timeout time.Duration, dir, token string, args ...string) ([]byte, error) {
	out, err := f.out, f.err
	if f.calls < len(f.outs) {
		out = f.outs[f.calls]
	}
	if f.calls < len(f.errs) {
		err = f.errs[f.calls]
	}
	f.calls++
	f.timeout, f.dir, f.token, f.args = timeout, dir, token, args
	f.history = append(f.history, args)
	return out, err
}

// withGH builds a Service whose gh calls land on the fake instead of the CLI.
func withGH(f *fakeGH) *Service { return &Service{runner: f.run} }

func TestPullRequestDetailFlow(t *testing.T) {
	t.Run("open PR decodes, scoped to the path", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"number":7,"state":"OPEN","title":"t"}`)}
		pr, err := withGH(gh).PullRequestDetail("/repo", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil || pr.Number != 7 {
			t.Fatalf("wrong detail: %+v", pr)
		}
		if gh.dir != "/repo" {
			t.Errorf("dir = %q, want /repo", gh.dir)
		}
		if want := []string{"pr", "view", "--json", prViewFields}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
		if gh.timeout != prReadTimeout {
			t.Errorf("timeout = %v, want %v", gh.timeout, prReadTimeout)
		}
	})

	t.Run("no pull request is an empty panel, not an error", func(t *testing.T) {
		pr, err := withGH(&fakeGH{err: errNoPullRequest}).PullRequestDetail("/repo", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr != nil {
			t.Errorf("expected nil detail, got %+v", pr)
		}
	})

	t.Run("a real gh failure reaches the caller as written", func(t *testing.T) {
		// The runner already turned gh's stderr into the screen's message
		// (gherror.go); the flow must pass it through rather than prefix it
		// with the command it ran.
		gh := &fakeGH{err: errors.New("gh is not signed in to GitHub.")}
		_, err := withGH(gh).PullRequestDetail("/repo", 0)
		if err == nil {
			t.Fatal("expected an error")
		}
		if err.Error() != "gh is not signed in to GitHub." {
			t.Errorf("error = %q, want the runner's message unchanged", err)
		}
	})

	t.Run("malformed gh output errors", func(t *testing.T) {
		if _, err := withGH(&fakeGH{out: []byte(`{not json`)}).PullRequestDetail("/repo", 0); err == nil {
			t.Error("expected a decode error")
		}
	})
}

// TestPullRequestBadgeRunsThroughTheRunner pins the badge to the same runner as
// the rest: it used to shell out to gh on its own, which left it running as
// whatever account gh had active while the screen beside it used the project's.
func TestPullRequestBadgeRunsThroughTheRunner(t *testing.T) {
	gh := &fakeGH{out: []byte(`{"number":7,"url":"https://github.com/o/r/pull/7","state":"OPEN"}`)}
	pr := withGH(gh).PullRequest("/repo")
	if pr == nil || pr.Number != 7 {
		t.Fatalf("PullRequest() = %+v, want the open PR", pr)
	}
	if want := []string{"pr", "view", "--json", "number,url,state"}; !slices.Equal(gh.args, want) {
		t.Errorf("args = %v, want %v", gh.args, want)
	}
	if gh.dir != "/repo" || gh.timeout != prLookupTimeout {
		t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
	}

	// A failure still hides the badge rather than surfacing anything.
	if pr := withGH(&fakeGH{err: errTest}).PullRequest("/repo"); pr != nil {
		t.Errorf("failed lookup = %+v, want nil", pr)
	}

	// And it runs as the project's account, like every other GitHub call.
	account := &accountGH{token: "gho_secret", pr: `{"number":7,"url":"u","state":"OPEN"}`}
	svc := &Service{runner: account.run}
	svc.SetAccounts(func(string) string { return "octocat" })
	if pr := svc.PullRequest("/repo"); pr == nil {
		t.Fatal("PullRequest() = nil, want the open PR")
	}
	if account.viewToken != "gho_secret" {
		t.Errorf("badge ran with token %q, want the configured account's", account.viewToken)
	}
}

func TestMergePullRequestFlow(t *testing.T) {
	t.Run("the built args reach gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).MergePullRequest("/repo", 0, "squash", "", "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "merge", "--squash"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
		if gh.dir != "/repo" || gh.timeout != prMergeTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("an unknown method never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).MergePullRequest("/repo", 0, "force", "", "", false); err == nil {
			t.Error("expected an error for an unknown merge method")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("gh's refusal surfaces", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("Pull request is not mergeable")}
		err := withGH(gh).MergePullRequest("/repo", 0, "merge", "", "", false)
		if err == nil {
			t.Fatal("expected an error")
		}
		if err.Error() != "Pull request is not mergeable" {
			t.Errorf("error = %q, want the runner's message unchanged", err)
		}
	})
}

func TestCreatePullRequestFlow(t *testing.T) {
	t.Run("opens the web flow in the path", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).CreatePullRequest("/repo"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "create", "--web"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
		if gh.dir != "/repo" || gh.timeout != prCreateTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("gh's failure surfaces", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("no git remote found")}
		err := withGH(gh).CreatePullRequest("/repo")
		if err == nil {
			t.Fatal("expected an error")
		}
		if err.Error() != "no git remote found" {
			t.Errorf("error = %q, want the runner's message unchanged", err)
		}
	})
}

func TestPullRequestDiffFlow(t *testing.T) {
	t.Run("plain diff text is returned verbatim", func(t *testing.T) {
		gh := &fakeGH{out: []byte("diff --git a/a.txt b/a.txt\n")}
		text, err := withGH(gh).PullRequestDiff("/repo", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if text != "diff --git a/a.txt b/a.txt\n" {
			t.Errorf("diff = %q", text)
		}
		if want := []string{"pr", "diff", "--color", "never"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
		if gh.dir != "/repo" {
			t.Errorf("dir = %q, want /repo", gh.dir)
		}
	})

	t.Run("no pull request yields an empty diff, not an error", func(t *testing.T) {
		text, err := withGH(&fakeGH{err: errNoPullRequest}).PullRequestDiff("/repo", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if text != "" {
			t.Errorf("diff = %q, want empty", text)
		}
	})

	t.Run("a real gh failure surfaces", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("could not resolve to a Repository")}
		if _, err := withGH(gh).PullRequestDiff("/repo", 0); err == nil {
			t.Fatal("expected an error")
		} else if err.Error() != "could not resolve to a Repository" {
			t.Errorf("error = %q, want the runner's message unchanged", err)
		}
	})
}

func TestPRArgs(t *testing.T) {
	tests := []struct {
		name   string
		number int
		want   []string
	}{
		// Zero is the branch's own PR: gh's default, and what every call site
		// did before a pull request could be addressed by number.
		{"no selector for the current branch", 0, []string{"pr", "view", "--json", "x"}},
		{"a negative number is no selector either", -1, []string{"pr", "view", "--json", "x"}},
		{"the number leads the flags", 9, []string{"pr", "view", "9", "--json", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prArgs("view", tt.number, "--json", "x"); !slices.Equal(got, tt.want) {
				t.Errorf("prArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPullRequestDetailByNumber(t *testing.T) {
	gh := &fakeGH{out: []byte(`{"number":42,"state":"OPEN","title":"t"}`)}
	pr, err := withGH(gh).PullRequestDetail("/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr == nil || pr.Number != 42 {
		t.Fatalf("wrong detail: %+v", pr)
	}
	if want := []string{"pr", "view", "42", "--json", prViewFields}; !slices.Equal(gh.args, want) {
		t.Errorf("args = %v, want %v", gh.args, want)
	}
}

func TestParsePRList(t *testing.T) {
	t.Run("rows carry the author, the fork flag and the rollup", func(t *testing.T) {
		out := []byte(`[
			{
				"number": 107,
				"title": "feat: list a project's open pull requests",
				"author": {"login": "omartelo"},
				"isDraft": false,
				"headRefName": "feat/pulls-list",
				"isCrossRepository": false,
				"updatedAt": "2026-07-26T12:00:00Z",
				"statusCheckRollup": [
					{"status": "COMPLETED", "conclusion": "SUCCESS"},
					{"status": "COMPLETED", "conclusion": "FAILURE"},
					{"status": "IN_PROGRESS"}
				]
			},
			{
				"number": 103,
				"title": "docs: fix the port number",
				"author": {"name": "Ada Lovelace"},
				"isDraft": true,
				"headRefName": "patch-1",
				"isCrossRepository": true,
				"updatedAt": "2026-07-24T09:30:00Z",
				"statusCheckRollup": []
			}
		]`)
		list, err := parsePRList(out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("want 2 rows, got %d", len(list))
		}
		first := list[0]
		if first.Number != 107 || first.Author != "omartelo" || first.HeadRefName != "feat/pulls-list" {
			t.Errorf("wrong first row: %+v", first)
		}
		want := ChecksRollup{Passed: 1, Failed: 1, Pending: 1, Total: 3}
		if first.Checks != want {
			t.Errorf("checks = %+v, want %+v", first.Checks, want)
		}
		// A fork PR still lists — only the session flow refuses it.
		if !list[1].IsCrossRepository || !list[1].IsDraft {
			t.Errorf("second row lost its flags: %+v", list[1])
		}
		// gh reports no login for some authors; the display name is the fallback.
		if list[1].Author != "Ada Lovelace" {
			t.Errorf("author fallback failed: %q", list[1].Author)
		}
	})

	t.Run("no open pull request is an empty column, not an error", func(t *testing.T) {
		list, err := parsePRList([]byte(`[]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if list != nil {
			t.Errorf("want nil, got %+v", list)
		}
	})

	t.Run("malformed output errors", func(t *testing.T) {
		if _, err := parsePRList([]byte(`{not json`)); err == nil {
			t.Error("expected a decode error")
		}
	})
}

func TestListPullRequestsFlow(t *testing.T) {
	t.Run("the call is scoped and bounded", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`[{"number":1,"title":"t"}]`)}
		list, err := withGH(gh).ListPullRequests("/repo", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("want 1 row, got %d", len(list))
		}
		want := []string{"pr", "list", "--state", "open", "--limit", "50", "--json", prListFields}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
		if gh.dir != "/repo" || gh.timeout != prReadTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("a real gh failure surfaces its cause", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("gh auth login required")}
		_, err := withGH(gh).ListPullRequests("/repo", "")
		if err == nil {
			t.Fatal("expected an error")
		}
		if err.Error() != "gh auth login required" {
			t.Errorf("error = %q, want the runner's message unchanged", err)
		}
	})

	t.Run("a known state reaches gh", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`[]`)}
		if _, err := withGH(gh).ListPullRequests("/repo", "merged"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !slices.Contains(gh.args, "merged") {
			t.Errorf("args = %v, want the state among them", gh.args)
		}
	})

	// The state comes from a query typed in the page, so anything outside the
	// allow-list has to stop here rather than reach gh's flags.
	t.Run("an unknown state never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).ListPullRequests("/repo", "--limit=99999"); err == nil {
			t.Error("expected an error for an unknown state")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

// TestParsePRDetailReview proves the aggregate review verdict survives the
// decode — the field the Pulls header reads to say a PR is approved.
func TestParsePRDetailReview(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"approved", `{"number":1,"state":"OPEN","reviewDecision":"APPROVED"}`, "APPROVED"},
		{
			"changes requested",
			`{"number":1,"state":"OPEN","reviewDecision":"CHANGES_REQUESTED"}`,
			"CHANGES_REQUESTED",
		},
		// A repository that requires no review reports nothing, which the
		// header treats as "no review to speak of" rather than as pending.
		{"none required", `{"number":1,"state":"OPEN"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := parsePRDetail([]byte(tt.raw), true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pr == nil || pr.ReviewDecision != tt.want {
				t.Errorf("ReviewDecision = %+v, want %q", pr, tt.want)
			}
		})
	}
}

// TestPullRequestDetailByNumberIgnoresState proves a number-addressed lookup
// returns a merged pull request. The branch lookup still gates them out — a
// merged PR must not keep showing on the badge — but a row the list offers has
// to open, and the list can be asked for merged and closed ones.
func TestPullRequestDetailByNumberIgnoresState(t *testing.T) {
	merged := []byte(`{"number":13960,"state":"MERGED","title":"Add macos keyring security doc"}`)

	pr, err := withGH(&fakeGH{out: merged}).PullRequestDetail("/repo", 13960)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr == nil || pr.State != "MERGED" {
		t.Fatalf("a numbered merged pull request should open: %+v", pr)
	}

	branch, err := withGH(&fakeGH{out: merged}).PullRequestDetail("/repo", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != nil {
		t.Errorf("the branch lookup must still hide a merged PR, got %+v", branch)
	}
}
