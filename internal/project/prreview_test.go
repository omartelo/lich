package project

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReviewPayload(t *testing.T) {
	t.Run("a plain approval carries no body and no comments", func(t *testing.T) {
		payload, err := reviewPayload("approve", "", "abc123", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Event != "APPROVE" || payload.CommitID != "abc123" {
			t.Errorf("payload = %+v", payload)
		}
		if payload.Comments != nil || payload.Body != "" {
			t.Errorf("expected an empty approval, got %+v", payload)
		}
	})

	t.Run("a single-line comment is a line, never a range", func(t *testing.T) {
		payload, err := reviewPayload("comment", "", "", []ReviewComment{
			{Path: "internal/project/pr.go", Line: 61, StartLine: 61, Body: "this buffers"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := payload.Comments[0]
		if got.Line != 61 || got.StartLine != 0 || got.StartSide != "" {
			t.Errorf("comment = %+v, want a bare line", got)
		}
		// An unset side is the new file: where all but a deletion comment goes.
		if got.Side != "RIGHT" {
			t.Errorf("side = %q, want RIGHT", got.Side)
		}
	})

	t.Run("a multi-line comment keeps both ends on the same side", func(t *testing.T) {
		payload, err := reviewPayload("request_changes", "two things", "", []ReviewComment{
			{Path: "a.go", Line: 61, StartLine: 58, Side: "LEFT", Body: "x"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := payload.Comments[0]
		if got.StartLine != 58 || got.Side != "LEFT" || got.StartSide != "LEFT" {
			t.Errorf("comment = %+v", got)
		}
	})

	t.Run("what GitHub would refuse never becomes a call", func(t *testing.T) {
		cases := []struct {
			name     string
			event    string
			body     string
			comments []ReviewComment
		}{
			{"unknown verdict", "merge-it", "x", nil},
			{"a review that says nothing", "comment", "", nil},
			{"changes requested with nothing to change", "request_changes", "", nil},
			{"a comment with no body", "comment", "", []ReviewComment{{Path: "a.go", Line: 3}}},
			{"a comment with no file", "comment", "", []ReviewComment{{Line: 3, Body: "x"}}},
			{"a comment with no line", "comment", "", []ReviewComment{{Path: "a.go", Body: "x"}}},
			{"a side that is neither", "comment", "", []ReviewComment{{Path: "a.go", Line: 3, Side: "MIDDLE", Body: "x"}}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := reviewPayload(tc.event, tc.body, "", tc.comments); err == nil {
					t.Error("expected a refusal before the shell-out")
				}
			})
		}
	})
}

func TestSubmitReviewFlow(t *testing.T) {
	t.Run("the review reaches gh as a JSON body, and the file does not outlive it", func(t *testing.T) {
		gh := &fakeGH{}
		comments := []ReviewComment{{Path: "a.go", Line: 4, Body: "why"}}
		if err := withGH(gh).SubmitReview("/repo", 42, "request_changes", "two things", "abc", comments); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gh.args) != 6 {
			t.Fatalf("args = %v", gh.args)
		}
		head := []string{"api", "--method", "POST", "repos/{owner}/{repo}/pulls/42/reviews", "--input"}
		if !slices.Equal(gh.args[:5], head) {
			t.Errorf("args = %v, want %v followed by a file", gh.args, head)
		}
		if _, err := os.Stat(gh.args[5]); !os.IsNotExist(err) {
			t.Errorf("staged file %q outlived the call", gh.args[5])
		}
		if gh.dir != "/repo" || gh.timeout != prReviewTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("the staged body is what GitHub expects", func(t *testing.T) {
		// The file is gone by the time the call returns, so the body is checked
		// where gh would read it: inside the runner.
		var staged []byte
		svc := &Service{runner: func(_ time.Duration, _, _ string, args ...string) ([]byte, error) {
			staged, _ = os.ReadFile(args[len(args)-1])
			return nil, nil
		}}
		err := svc.SubmitReview("/repo", 7, "approve", "", "deadbeef", []ReviewComment{
			{Path: "a.go", Line: 9, StartLine: 7, Body: "here"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(staged, &body); err != nil {
			t.Fatalf("staged body is not JSON: %v (%s)", err, staged)
		}
		if body["event"] != "APPROVE" || body["commit_id"] != "deadbeef" {
			t.Errorf("body = %v", body)
		}
		comments, _ := body["comments"].([]any)
		if len(comments) != 1 {
			t.Fatalf("comments = %v", body["comments"])
		}
		comment, _ := comments[0].(map[string]any)
		if comment["start_line"] != float64(7) || comment["start_side"] != "RIGHT" {
			t.Errorf("comment = %v, want GitHub's snake_case range", comment)
		}
	})

	t.Run("an invalid review never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).SubmitReview("/repo", 1, "nope", "", "", nil); err == nil {
			t.Error("expected an error for an unknown verdict")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("GitHub's refusal reaches the caller as written", func(t *testing.T) {
		gh := &fakeGH{err: testError("GitHub does not let an account approve its own pull request.")}
		err := withGH(gh).SubmitReview("/repo", 7, "approve", "", "", nil)
		if err == nil || !strings.Contains(err.Error(), "approve its own") {
			t.Errorf("error = %v, want the runner's message unchanged", err)
		}
	})
}

func TestReplyToReviewThreadFlow(t *testing.T) {
	t.Run("a reply is addressed to the comment's database id", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).ReplyToReviewThread("/repo", 42, 900001, "dropped it"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"api", "--method", "POST",
			"repos/{owner}/{repo}/pulls/42/comments/900001/replies",
			"-f", "body=dropped it",
		}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("an empty reply never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).ReplyToReviewThread("/repo", 42, 1, ""); err == nil {
			t.Error("expected an error for an empty reply")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestResolveReviewThreadFlow(t *testing.T) {
	t.Run("resolving and reopening are two mutations, one node id", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).ResolveReviewThread("/repo", "PRRT_kw1", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"api", "graphql", "-f", "query=" + resolveThreadMutation, "-f", "id=PRRT_kw1"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}

		if err := withGH(gh).ResolveReviewThread("/repo", "PRRT_kw1", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(gh.args[3], "unresolveReviewThread") {
			t.Errorf("args = %v, want the unresolve mutation", gh.args)
		}
	})

	t.Run("no thread never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).ResolveReviewThread("/repo", "", true); err == nil {
			t.Error("expected an error without a thread")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestCommentOnPullRequestFlow(t *testing.T) {
	t.Run("a numbered pull request is addressed by number", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).CommentOnPullRequest("/repo", 42, "rebased"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "comment", "42", "--body", "rebased"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("an empty comment never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).CommentOnPullRequest("/repo", 42, ""); err == nil {
			t.Error("expected an error for an empty comment")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}
