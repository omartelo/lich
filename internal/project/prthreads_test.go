package project

import (
	"slices"
	"strings"
	"testing"
)

const conversationPayload = `{"data":{"repository":{"pullRequest":{
  "headRefOid": "21e6cdc4336e8f5db46724d82fecf0423bc41a0a",
  "reviews": {"nodes": [
    {"author":{"login":"omartelo"},"state":"CHANGES_REQUESTED","body":"two things first","submittedAt":"2026-07-30T10:00:00Z"},
    {"author":{"login":"bot"},"state":"COMMENTED","body":"","submittedAt":"2026-07-30T10:05:00Z"},
    {"author":{"login":"drafting"},"state":"PENDING","body":"not submitted","submittedAt":null},
    {"author":{"login":"reviewer"},"state":"APPROVED","body":"","submittedAt":"2026-07-30T11:00:00Z"}
  ]},
  "comments": {"nodes": [
    {"author":{"login":"omartelo"},"body":"rebased on main","createdAt":"2026-07-30T09:00:00Z"}
  ]},
  "reviewThreads": {"nodes": [
    {"id":"PRRT_kw1","isResolved":false,"isOutdated":false,"path":"internal/project/prthreads.go",
     "line":61,"originalLine":58,"startLine":59,"diffSide":"RIGHT",
     "comments":{"nodes":[
       {"databaseId":900001,"author":{"login":"omartelo"},"body":"this buffers every hunk","createdAt":"2026-07-30T10:01:00Z","diffHunk":"@@ -57,6 +57,10 @@"},
       {"databaseId":900002,"author":{"login":"claude"},"body":"dropped it","createdAt":"2026-07-30T10:30:00Z","diffHunk":"@@ -57,6 +57,10 @@"}
     ]}},
    {"id":"PRRT_kw2","isResolved":true,"isOutdated":true,"path":"frontend/src/lib/pulls/hook.ts",
     "line":null,"originalLine":18,"startLine":null,"diffSide":"RIGHT",
     "comments":{"nodes":[
       {"databaseId":900003,"author":null,"body":"gone account","createdAt":"2026-07-30T08:00:00Z","diffHunk":"@@ -16,3 +16,5 @@"}
     ]}},
    {"id":"PRRT_kw3","isResolved":false,"isOutdated":true,"path":"internal/store/store.go",
     "line":null,"originalLine":74,"startLine":null,"originalStartLine":65,"diffSide":"RIGHT",
     "comments":{"nodes":[
       {"databaseId":900004,"author":{"login":"reviewer"},"body":"this range","createdAt":"2026-07-30T09:30:00Z","diffHunk":"@@ -60,0 +60,15 @@"}
     ]}}
  ]}
}}}}`

func TestParseConversation(t *testing.T) {
	convo, err := parseConversation([]byte(conversationPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if convo.HeadRefOID != "21e6cdc4336e8f5db46724d82fecf0423bc41a0a" {
		t.Errorf("head = %q, want the queried oid", convo.HeadRefOID)
	}

	t.Run("an empty COMMENTED review and a pending one are dropped", func(t *testing.T) {
		if len(convo.Reviews) != 2 {
			t.Fatalf("reviews = %+v, want the two that say something", convo.Reviews)
		}
		if convo.Reviews[0].State != "CHANGES_REQUESTED" || convo.Reviews[0].Body != "two things first" {
			t.Errorf("first review = %+v", convo.Reviews[0])
		}
		// An approval with no body still carries its verdict.
		if convo.Reviews[1].State != "APPROVED" || convo.Reviews[1].Author != "reviewer" {
			t.Errorf("second review = %+v", convo.Reviews[1])
		}
	})

	t.Run("pull request comments come through", func(t *testing.T) {
		if len(convo.Comments) != 1 || convo.Comments[0].Body != "rebased on main" {
			t.Fatalf("comments = %+v", convo.Comments)
		}
		if convo.Comments[0].Author != "omartelo" || convo.Comments[0].Date == "" {
			t.Errorf("comment lost its byline: %+v", convo.Comments[0])
		}
	})

	t.Run("an anchored thread keeps its line span and both ids", func(t *testing.T) {
		if len(convo.Threads) != 3 {
			t.Fatalf("threads = %d, want 3", len(convo.Threads))
		}
		anchored := convo.Threads[0]
		if anchored.ID != "PRRT_kw1" || anchored.Line != 61 || anchored.StartLine != 59 {
			t.Errorf("anchor = %+v", anchored)
		}
		if anchored.Side != "RIGHT" || anchored.IsResolved || anchored.IsOutdated {
			t.Errorf("flags = %+v", anchored)
		}
		if len(anchored.Comments) != 2 {
			t.Fatalf("thread comments = %+v", anchored.Comments)
		}
		// The reply address is the comment's database id, never the node id.
		if anchored.Comments[0].ID != 900001 || anchored.Comments[1].ID != 900002 {
			t.Errorf("comment ids = %+v", anchored.Comments)
		}
		if anchored.Comments[0].DiffHunk == "" {
			t.Error("expected the hunk to travel with the comment")
		}
	})

	t.Run("an outdated thread falls back to its original line", func(t *testing.T) {
		outdated := convo.Threads[1]
		if !outdated.IsOutdated || !outdated.IsResolved {
			t.Errorf("flags = %+v", outdated)
		}
		if outdated.Line != 18 {
			t.Errorf("line = %d, want the original 18 that GitHub kept", outdated.Line)
		}
		if outdated.StartLine != 0 {
			t.Errorf("startLine = %d, want 0 for a single-line thread", outdated.StartLine)
		}
	})

	t.Run("an outdated range keeps its original first line", func(t *testing.T) {
		// Without it the span collapses to one line, and the snippet the
		// Conversation tab cuts from the hunk loses the lines it was filed on.
		ranged := convo.Threads[2]
		if ranged.StartLine != 65 || ranged.Line != 74 {
			t.Errorf("span = %d–%d, want the original 65–74", ranged.StartLine, ranged.Line)
		}
	})

	t.Run("a deleted account leaves an empty login, not a crash", func(t *testing.T) {
		if got := convo.Threads[1].Comments[0].Author; got != "" {
			t.Errorf("author = %q, want empty", got)
		}
	})
}

func TestParseConversationEdges(t *testing.T) {
	t.Run("a pull request nobody has said anything about", func(t *testing.T) {
		convo, err := parseConversation([]byte(`{"data":{"repository":{"pullRequest":{
			"headRefOid":"abc","reviews":{"nodes":[]},"comments":{"nodes":[]},"reviewThreads":{"nodes":[]}}}}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if convo.Reviews != nil || convo.Comments != nil || convo.Threads != nil {
			t.Errorf("empty conversation should be nil throughout, got %+v", convo)
		}
	})

	t.Run("an unreadable answer is a sentence, not a decoder dump", func(t *testing.T) {
		_, err := parseConversation([]byte(`{"data":`))
		if err == nil {
			t.Fatal("expected a decode error")
		}
		if !strings.HasSuffix(err.Error(), ".") {
			t.Errorf("error = %q, want the screen's sentence", err)
		}
	})
}

func TestPullRequestConversationFlow(t *testing.T) {
	t.Run("the query is addressed by number and scoped to the path", func(t *testing.T) {
		gh := &fakeGH{out: []byte(conversationPayload)}
		convo, err := withGH(gh).PullRequestConversation("/repo", 104)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if convo == nil || len(convo.Threads) != 3 {
			t.Fatalf("wrong conversation: %+v", convo)
		}
		want := []string{
			"api", "graphql",
			"-F", "owner={owner}", "-F", "repo={repo}",
			"-F", "number=104",
			"-f", "query=" + prConversationQuery,
		}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v,\nwant %v", gh.args, want)
		}
		if gh.dir != "/repo" || gh.timeout != prReadTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("no number never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).PullRequestConversation("/repo", 0); err == nil {
			t.Error("expected an error without a pull request number")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("a gh failure reaches the caller as written", func(t *testing.T) {
		gh := &fakeGH{err: testError("GitHub is rate-limiting this account. Try again in a few minutes.")}
		_, err := withGH(gh).PullRequestConversation("/repo", 7)
		if err == nil || err.Error() != "GitHub is rate-limiting this account. Try again in a few minutes." {
			t.Errorf("error = %v, want the runner's message unchanged", err)
		}
	})
}
