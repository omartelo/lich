package project

import (
	"encoding/json"
	"fmt"
)

// A pull request's conversation is the one part of it gh's own `pr view` cannot
// answer: --json reviews carries a review's body but never the threads hanging
// off its lines, and no REST call reports whether a thread was resolved. The
// GraphQL API has all three in one round-trip, so that is what this asks for.
//
// The account resolution is the same as every other gh call (ghaccount.go): the
// query runs inside the checkout, and gh substitutes {owner}/{repo} from its
// remote.
const prConversationQuery = `query($owner:String!,$repo:String!,$number:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$number){
      headRefOid
      reviews(last:50){nodes{author{login} state body submittedAt}}
      comments(last:50){nodes{author{login} body createdAt}}
      reviewThreads(last:100){nodes{
        id isResolved isOutdated path line originalLine startLine originalStartLine diffSide
        comments(first:50){nodes{databaseId author{login} body createdAt diffHunk}}
      }}
    }
  }
}`

// PRConversation is everything said about a pull request: the reviews and their
// verdicts, the comments left on the pull request itself, and the threads
// anchored to a file and line. The Pulls screen renders the anchored ones inside
// the diff and the rest in its Conversation tab.
type PRConversation struct {
	// HeadRefOID is the commit a new review comment is filed against. GitHub
	// defaults to the latest commit, but naming it means a review submitted
	// seconds after a push cannot silently land on lines nobody read.
	HeadRefOID string     `json:"headRefOid"`
	Reviews    []PRReview `json:"reviews"`
	// Comments are the pull request's own comments — the ones with no file and
	// no line, which GitHub files as issue comments.
	Comments []PRComment      `json:"comments"`
	Threads  []PRReviewThread `json:"threads"`
}

// PRReview is one submitted review: its verdict, and whatever was written in the
// summary box above the line comments.
type PRReview struct {
	Author string `json:"author"`
	// State is gh's APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED.
	State string `json:"state"`
	Body  string `json:"body"`
	Date  string `json:"date"`
}

// PRComment is one comment on the pull request itself.
type PRComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	Date   string `json:"date"`
}

// PRReviewThread is one conversation anchored to a line of the diff.
type PRReviewThread struct {
	// ID is the GraphQL node id, which is what resolving a thread takes. It is
	// not the number a reply is addressed to — that is a comment's own ID.
	ID   string `json:"id"`
	Path string `json:"path"`
	// Line is the anchor in the file the thread's side names, and StartLine the
	// first line of a multi-line thread (0 when it covers one line). A thread
	// GitHub could not re-anchor keeps its original line and is marked outdated,
	// which is what sends it to the Conversation tab instead of the diff.
	Line      int `json:"line"`
	StartLine int `json:"startLine"`
	// Side is RIGHT for a thread on the new file, LEFT for one on a deleted
	// line — the two sides the diff renders in one document.
	Side       string            `json:"side"`
	IsResolved bool              `json:"isResolved"`
	IsOutdated bool              `json:"isOutdated"`
	Comments   []PRThreadComment `json:"comments"`
}

// PRThreadComment is one message inside a thread.
type PRThreadComment struct {
	// ID is the REST database id — the address a reply is posted to. GraphQL
	// node ids do not work there, which is why both identifiers travel.
	ID     int64  `json:"id"`
	Author string `json:"author"`
	Body   string `json:"body"`
	Date   string `json:"date"`
	// DiffHunk is GitHub's own snippet of the lines the thread is about. The
	// diff already has them where the thread is anchored; this is what an
	// outdated thread has left.
	DiffHunk string `json:"diffHunk"`
}

// ghConversation mirrors the GraphQL payload; nothing below leaves this package
// in this shape.
type ghConversation struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				HeadRefOID string `json:"headRefOid"`
				Reviews    struct {
					Nodes []ghReview `json:"nodes"`
				} `json:"reviews"`
				Comments struct {
					Nodes []ghIssueComment `json:"nodes"`
				} `json:"comments"`
				ReviewThreads struct {
					Nodes []ghReviewThread `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// ghActor is GitHub's author object, which is null for an account that has been
// deleted since — the zero value then reads as an empty login.
type ghActor struct {
	Login string `json:"login"`
}

type ghReview struct {
	Author      ghActor `json:"author"`
	State       string  `json:"state"`
	Body        string  `json:"body"`
	SubmittedAt string  `json:"submittedAt"`
}

type ghIssueComment struct {
	Author    ghActor `json:"author"`
	Body      string  `json:"body"`
	CreatedAt string  `json:"createdAt"`
}

type ghReviewThread struct {
	ID                string `json:"id"`
	IsResolved        bool   `json:"isResolved"`
	IsOutdated        bool   `json:"isOutdated"`
	Path              string `json:"path"`
	Line              *int   `json:"line"`
	OriginalLine      *int   `json:"originalLine"`
	StartLine         *int   `json:"startLine"`
	OriginalStartLine *int   `json:"originalStartLine"`
	DiffSide          string `json:"diffSide"`
	Comments          struct {
		Nodes []ghThreadComment `json:"nodes"`
	} `json:"comments"`
}

type ghThreadComment struct {
	DatabaseID int64   `json:"databaseId"`
	Author     ghActor `json:"author"`
	Body       string  `json:"body"`
	CreatedAt  string  `json:"createdAt"`
	DiffHunk   string  `json:"diffHunk"`
}

// PullRequestConversation reads one pull request's reviews, comments and review
// threads. The number is required: the query addresses a pull request by number,
// and the screen always knows it by the time there is a conversation to show —
// resolving "the branch's PR" here would cost a second gh round-trip on every
// read.
func (s *Service) PullRequestConversation(path string, number int) (*PRConversation, error) {
	if number <= 0 {
		return nil, fmt.Errorf("a pull request number is required")
	}
	out, err := s.gh(prReadTimeout, path, "api", "graphql",
		"-F", "owner={owner}", "-F", "repo={repo}",
		"-F", fmt.Sprintf("number=%d", number),
		"-f", "query="+prConversationQuery)
	if err != nil {
		return nil, err
	}
	return parseConversation(out)
}

// parseConversation reduces the GraphQL payload to what the screen renders.
func parseConversation(out []byte) (*PRConversation, error) {
	var payload ghConversation
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	pr := payload.Data.Repository.PullRequest
	return &PRConversation{
		HeadRefOID: pr.HeadRefOID,
		Reviews:    toReviews(pr.Reviews.Nodes),
		Comments:   toComments(pr.Comments.Nodes),
		Threads:    toThreads(pr.ReviewThreads.Nodes),
	}, nil
}

// toReviews keeps the reviews that say something. A review with no body and no
// verdict is the envelope GitHub wraps line comments in — its threads are
// rendered on their own lines, and the empty shell would read as a person who
// reviewed and wrote nothing. A pending review is one nobody has submitted yet.
func toReviews(nodes []ghReview) []PRReview {
	out := make([]PRReview, 0, len(nodes))
	for _, r := range nodes {
		if r.State == "PENDING" || (r.Body == "" && r.State == "COMMENTED") {
			continue
		}
		out = append(out, PRReview{
			Author: r.Author.Login,
			State:  r.State,
			Body:   r.Body,
			Date:   r.SubmittedAt,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func toComments(nodes []ghIssueComment) []PRComment {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRComment, 0, len(nodes))
	for _, c := range nodes {
		out = append(out, PRComment{Author: c.Author.Login, Body: c.Body, Date: c.CreatedAt})
	}
	return out
}

func toThreads(nodes []ghReviewThread) []PRReviewThread {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRReviewThread, 0, len(nodes))
	for _, t := range nodes {
		out = append(out, PRReviewThread{
			ID:         t.ID,
			Path:       t.Path,
			Line:       threadLine(t),
			StartLine:  threadStartLine(t),
			Side:       t.DiffSide,
			IsResolved: t.IsResolved,
			IsOutdated: t.IsOutdated,
			Comments:   toThreadComments(t.Comments.Nodes),
		})
	}
	return out
}

// threadLine is where the thread hangs. GitHub nulls `line` for a thread whose
// lines the branch has since rewritten and keeps the original — a number that no
// longer addresses anything in the current diff, which is exactly why such a
// thread is marked outdated and rendered away from the file.
func threadLine(t ghReviewThread) int {
	if t.Line != nil {
		return *t.Line
	}
	return deref(t.OriginalLine)
}

// threadStartLine is the same fallback for the range's first line: an outdated
// thread keeps the original, which is the number its diff hunk is written in.
func threadStartLine(t ghReviewThread) int {
	if t.StartLine != nil {
		return *t.StartLine
	}
	return deref(t.OriginalStartLine)
}

func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func toThreadComments(nodes []ghThreadComment) []PRThreadComment {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRThreadComment, 0, len(nodes))
	for _, c := range nodes {
		out = append(out, PRThreadComment{
			ID:       c.DatabaseID,
			Author:   c.Author.Login,
			Body:     c.Body,
			Date:     c.CreatedAt,
			DiffHunk: c.DiffHunk,
		})
	}
	return out
}
