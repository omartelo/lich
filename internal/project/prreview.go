package project

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// The three writes a review takes, and why each one is the API it is:
//
//   - a reply belongs to a thread, and REST addresses one by the database id of
//     the comment that opened it;
//   - resolving a thread has no REST endpoint at all, only the GraphQL mutation;
//   - a review lands as one call carrying every line comment at once, which is
//     what makes it a review rather than a pile of separate remarks.
const (
	resolveThreadMutation = `mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}`

	unresolveThreadMutation = `mutation($id:ID!){unresolveReviewThread(input:{threadId:$id}){thread{isResolved}}}`
)

// reviewEvents allow-lists what a submitted review can say, and doubles as the
// map onto GitHub's own wording: an unknown verdict is rejected before any
// shell-out.
var reviewEvents = map[string]string{
	"approve":         "APPROVE",
	"comment":         "COMMENT",
	"request_changes": "REQUEST_CHANGES",
}

// reviewSides allow-lists which side of the diff a comment may hang off, so a
// value built in the page can never reach GitHub as something else.
var reviewSides = map[string]bool{"RIGHT": true, "LEFT": true}

// ReviewComment is one line comment of a review being submitted. Line is the
// last line it covers and StartLine the first (0 for a single-line comment) —
// GitHub's own convention, kept here so the page can hand over a selection
// unchanged.
type ReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	StartLine int    `json:"startLine"`
	// Side is RIGHT (the new file) or LEFT (a deleted line). Empty means RIGHT,
	// which is where all but a deletion comment goes.
	Side string `json:"side"`
	Body string `json:"body"`
}

// ghReviewPayload is GitHub's REST body for a submitted review. Its field names
// are snake_case and its comment shape is not lich's, which is the whole reason
// this type exists next to ReviewComment.
type ghReviewPayload struct {
	CommitID string            `json:"commit_id,omitempty"`
	Body     string            `json:"body,omitempty"`
	Event    string            `json:"event"`
	Comments []ghReviewComment `json:"comments,omitempty"`
}

type ghReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Body      string `json:"body"`
}

// SubmitReview files one review on GitHub: the verdict, the summary body, and
// every line comment the page has been holding, in a single call. head is the
// commit the comments were written against — empty lets GitHub assume its own
// latest, which is wrong the moment a push landed while the review was open.
//
// This is also the plain Approve: no comments, no body, event "approve". GitHub
// refuses an approval of one's own pull request, which the message table turns
// into a sentence (gherror.go).
func (s *Service) SubmitReview(path string, number int, event, body, head string, comments []ReviewComment) error {
	payload, err := reviewPayload(event, body, head, comments)
	if err != nil {
		return err
	}
	file, err := writeTempJSON(payload)
	if err != nil {
		return err
	}
	defer os.Remove(file)
	_, err = s.gh(prReviewTimeout, path, "api", "--method", "POST",
		"repos/{owner}/{repo}/pulls/"+strconv.Itoa(number)+"/reviews", "--input", file)
	return err
}

// reviewPayload validates the review and renders GitHub's body for it. The
// checks are here rather than at the call site because every one of them is a
// round-trip GitHub would refuse anyway: an unknown verdict, a side that is
// neither of the two, or a review that says nothing at all — which GitHub
// rejects for anything but an approval.
func reviewPayload(event, body, head string, comments []ReviewComment) (*ghReviewPayload, error) {
	verdict, ok := reviewEvents[event]
	if !ok {
		return nil, fmt.Errorf("unknown review verdict %q", event)
	}
	if verdict != "APPROVE" && body == "" && len(comments) == 0 {
		return nil, fmt.Errorf("a review with no verdict of its own needs a comment or a body")
	}
	out := &ghReviewPayload{CommitID: head, Body: body, Event: verdict}
	for _, c := range comments {
		comment, err := toGHReviewComment(c)
		if err != nil {
			return nil, err
		}
		out.Comments = append(out.Comments, comment)
	}
	return out, nil
}

// toGHReviewComment renders one line comment. A start line equal to the line, or
// past it, is not a range — GitHub rejects such a pair, and a one-line selection
// arrives here as exactly that.
func toGHReviewComment(c ReviewComment) (ghReviewComment, error) {
	side := c.Side
	if side == "" {
		side = "RIGHT"
	}
	if !reviewSides[side] {
		return ghReviewComment{}, fmt.Errorf("unknown diff side %q", c.Side)
	}
	if c.Path == "" || c.Line <= 0 {
		return ghReviewComment{}, fmt.Errorf("a review comment needs a file and a line")
	}
	if c.Body == "" {
		return ghReviewComment{}, fmt.Errorf("a review comment needs something written in it")
	}
	out := ghReviewComment{Path: c.Path, Line: c.Line, Side: side, Body: c.Body}
	if c.StartLine > 0 && c.StartLine < c.Line {
		out.StartLine, out.StartSide = c.StartLine, side
	}
	return out, nil
}

// writeTempJSON hands gh a body through a file because the runner (pr.go) has no
// stdin: gh's own --field flags cannot express an array of objects, and widening
// every gh call site with a stdin argument to serve this one call is a worse
// trade than a file that is removed as soon as the call returns.
func writeTempJSON(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not build the review: %w", err)
	}
	file, err := os.CreateTemp("", "lich-review-*.json")
	if err != nil {
		return "", fmt.Errorf("could not stage the review: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("could not stage the review: %w", err)
	}
	return file.Name(), nil
}

// ReplyToReviewThread answers an existing thread. commentID is the database id
// of any comment in it — GitHub files the reply against the thread that comment
// belongs to. A reply posts immediately and is never part of the pending review:
// it is an answer to something already said, which is the same split GitHub
// draws between "add single comment" and "start a review".
func (s *Service) ReplyToReviewThread(path string, number int, commentID int64, body string) error {
	if body == "" {
		return fmt.Errorf("a reply needs something written in it")
	}
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments/%d/replies", number, commentID)
	_, err := s.gh(prReviewTimeout, path, "api", "--method", "POST", endpoint, "-f", "body="+body)
	return err
}

// ResolveReviewThread marks a thread resolved, or reopens it. threadID is the
// GraphQL node id the conversation read carries — the database id a reply uses
// does not address a thread here.
func (s *Service) ResolveReviewThread(path, threadID string, resolved bool) error {
	if threadID == "" {
		return fmt.Errorf("a thread is required")
	}
	mutation := unresolveThreadMutation
	if resolved {
		mutation = resolveThreadMutation
	}
	_, err := s.gh(prReviewTimeout, path, "api", "graphql", "-f", "query="+mutation, "-f", "id="+threadID)
	return err
}

// CommentOnPullRequest leaves a comment on the pull request itself — the
// conversation with no file and no line behind it.
func (s *Service) CommentOnPullRequest(path string, number int, body string) error {
	if body == "" {
		return fmt.Errorf("a comment needs something written in it")
	}
	_, err := s.gh(prReviewTimeout, path, prArgs("comment", number, "--body", body)...)
	return err
}
