package project

import (
	"errors"
	"slices"
	"testing"
)

func TestToReviewers(t *testing.T) {
	t.Run("the two halves merge into one roster", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{Author: ghPRAuthor{Login: "ana"}, State: "APPROVED"}},
			[]ghReviewRequest{{TypeName: "User", Login: "bo"}},
		)
		want := []PRReviewer{
			{Login: "ana", State: "APPROVED"},
			{Login: "bo"},
		}
		if !slices.Equal(reviewers, want) {
			t.Errorf("wrong roster: %+v", reviewers)
		}
	})

	// The one case the merge exists for: GitHub keeps the old verdict in
	// latestReviews when a reviewer is asked again, and what stands is the answer
	// now owed — a roster still reading "approved" would hide the re-request.
	t.Run("a re-request outranks the verdict it follows", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{Author: ghPRAuthor{Login: "ana"}, State: "CHANGES_REQUESTED"}},
			[]ghReviewRequest{{TypeName: "User", Login: "ana"}},
		)
		if len(reviewers) != 1 {
			t.Fatalf("expected one entry, got %+v", reviewers)
		}
		if reviewers[0].Login != "ana" || reviewers[0].State != "" {
			t.Errorf("expected ana pending, got %+v", reviewers[0])
		}
	})

	t.Run("a team rides its slug and is marked as one", func(t *testing.T) {
		reviewers := toReviewers(nil, []ghReviewRequest{{TypeName: "Team", Slug: "platform"}})
		if len(reviewers) != 1 || reviewers[0].Login != "platform" || !reviewers[0].IsTeam {
			t.Errorf("wrong team entry: %+v", reviewers)
		}
	})

	t.Run("a deleted account leaves no nameless entry", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{State: "APPROVED"}},
			[]ghReviewRequest{{TypeName: "User"}},
		)
		if reviewers != nil {
			t.Errorf("expected no roster, got %+v", reviewers)
		}
	})

	t.Run("nothing on either side is no roster", func(t *testing.T) {
		if reviewers := toReviewers(nil, nil); reviewers != nil {
			t.Errorf("expected nil, got %+v", reviewers)
		}
	})
}

func TestParsePRDetailReviewers(t *testing.T) {
	out := []byte(`{
		"number": 7,
		"state": "OPEN",
		"author": {"login": "ana"},
		"latestReviews": [{"author": {"login": "bo"}, "state": "APPROVED"}],
		"reviewRequests": [{"__typename": "User", "login": "cai"}]
	}`)
	pr, err := parsePRDetail(out, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr == nil {
		t.Fatal("expected a detail, got nil")
	}
	if pr.Author != "ana" {
		t.Errorf("wrong author: %q", pr.Author)
	}
	want := []PRReviewer{{Login: "bo", State: "APPROVED"}, {Login: "cai"}}
	if !slices.Equal(pr.Reviewers, want) {
		t.Errorf("wrong roster: %+v", pr.Reviewers)
	}
}

func TestAssignableReviewers(t *testing.T) {
	t.Run("the nodes are the picker's list", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"data":{"repository":{"assignableUsers":{"nodes":[
			{"login":"ana","name":"Ana"},{"login":"bo","name":""}]}}}}`)}
		people, err := withGH(gh).AssignableReviewers("/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []PRCandidate{{Login: "ana", Name: "Ana"}, {Login: "bo"}}
		if !slices.Equal(people, want) {
			t.Errorf("wrong list: %+v", people)
		}
		if gh.dir != "/repo" {
			t.Errorf("wrong directory: %q", gh.dir)
		}
	})

	t.Run("a repository with nobody answers nothing", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"data":{"repository":{"assignableUsers":{"nodes":[]}}}}`)}
		people, err := withGH(gh).AssignableReviewers("/repo")
		if err != nil || people != nil {
			t.Errorf("expected an empty answer, got %+v (%v)", people, err)
		}
	})

	t.Run("gh's failure travels", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("gh is not signed in to GitHub.")}
		if _, err := withGH(gh).AssignableReviewers("/repo"); err == nil {
			t.Error("expected an error")
		}
	})

	t.Run("an unreadable answer is an error, not an empty list", func(t *testing.T) {
		if _, err := withGH(&fakeGH{out: []byte(`{not json`)}).AssignableReviewers("/repo"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestRequestReview(t *testing.T) {
	t.Run("asking adds the reviewer to the numbered pull request", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).RequestReview("/repo", 12, "ana", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"pr", "edit", "12", "--add-reviewer", "ana"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("wrong args: %v", gh.args)
		}
	})

	t.Run("withdrawing removes them", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).RequestReview("/repo", 12, "ana", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"pr", "edit", "12", "--remove-reviewer", "ana"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("wrong args: %v", gh.args)
		}
	})

	// Nothing reaches gh without a name: an empty one would leave the flag
	// holding an argument nobody meant to send.
	t.Run("no reviewer, no call", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).RequestReview("/repo", 12, "", true); err == nil {
			t.Error("expected an error")
		}
		if gh.calls != 0 {
			t.Errorf("expected no gh call, got %d", gh.calls)
		}
	})
}

func TestEditPullRequestBody(t *testing.T) {
	t.Run("the body replaces the description of the numbered PR", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).EditPullRequestBody("/repo", 12, "what it does"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"pr", "edit", "12", "--body", "what it does"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("wrong args: %v", gh.args)
		}
	})

	// Clearing the description is an edit like any other, and gh applies the
	// flag it was given rather than skipping an empty one.
	t.Run("an empty body still goes out", func(t *testing.T) {
		gh := &fakeGH{}
		if err := withGH(gh).EditPullRequestBody("/repo", 12, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"pr", "edit", "12", "--body", ""}
		if !slices.Equal(gh.args, want) {
			t.Errorf("wrong args: %v", gh.args)
		}
	})

	t.Run("gh's refusal travels", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("could not resolve to a Repository")}
		if err := withGH(gh).EditPullRequestBody("/repo", 12, "x"); err == nil {
			t.Error("expected an error")
		}
	})
}
