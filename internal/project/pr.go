package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/winexec"
)

// gh network calls are capped so a slow forge or a hung auth prompt never
// wedges the pull request screen. The reads (view, diff) share one budget;
// merge and create get more room because they push and mutate on the remote.
const (
	prReadTimeout   = 8 * time.Second
	prMergeTimeout  = 30 * time.Second
	prCreateTimeout = 20 * time.Second
)

// ghRunner runs one gh subcommand inside dir and returns its stdout. It is a
// field on Service so the pull request flows can be exercised without a GitHub
// remote; production wiring is runGH.
type ghRunner func(timeout time.Duration, dir string, args ...string) ([]byte, error)

// errNoPullRequest marks gh's "no pull requests found" — the one failure that
// means the branch simply has no PR, which the read flows turn into an empty
// panel instead of an error.
var errNoPullRequest = errors.New("no pull request for this branch")

// runGH shells out to gh with the call capped by timeout, and turns a failure
// into an error carrying gh's own stderr (it names the cause — not mergeable,
// branch protection, auth, missing repo).
func runGH(timeout time.Duration, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	winexec.Hide(cmd)
	out, err := cmd.Output()
	if err != nil {
		if isNoPullRequest(stderr.String()) {
			return nil, errNoPullRequest
		}
		return nil, errors.New(ghError(stderr.String(), err))
	}
	return out, nil
}

// prViewFields is the gh `pr view --json` selection backing the Pulls panel.
const prViewFields = "number,url,state,title,body,isDraft,mergeable,baseRefName,headRefName,statusCheckRollup,changedFiles,commits"

// PRDetail is the full view of a branch's open pull request — richer than the
// footer badge's PullRequest — driving the dock's Pulls panel: the title, body,
// CI rollup and mergeability gate the merge affordance. gh's state does not
// travel with it: a detail exists only when the PR is open (parsePRDetail).
type PRDetail struct {
	Number       int          `json:"number"`
	URL          string       `json:"url"`
	Title        string       `json:"title"`
	Body         string       `json:"body"`
	IsDraft      bool         `json:"isDraft"`
	Mergeable    string       `json:"mergeable"` // gh: MERGEABLE | CONFLICTING | UNKNOWN
	BaseRefName  string       `json:"baseRefName"`
	HeadRefName  string       `json:"headRefName"`
	ChangedFiles int          `json:"changedFiles"`
	Checks       ChecksRollup `json:"checks"`
	CheckRuns    []CheckItem  `json:"checkRuns"`
	Commits      []PRCommit   `json:"commits"`
}

// PRCommit is one commit the pull request would land, split the way git itself
// splits a message: the subject line, then the body (empty for a one-liner).
type PRCommit struct {
	OID      string `json:"oid"`
	Headline string `json:"headline"`
	Body     string `json:"body"`
	Author   string `json:"author"`
	Date     string `json:"date"` // gh's ISO committedDate
}

// ChecksRollup collapses gh's statusCheckRollup array into the counts the panel
// shows. Total is every check, so "all green" is Passed == Total && Total > 0.
type ChecksRollup struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
	Total   int `json:"total"`
}

// CheckItem is one check, flattened from gh's two rollup shapes into what the
// Checks tab lists. State is this package's own passed|failed|pending, the same
// verdict ChecksRollup counts. The timestamps are gh's ISO strings, left for the
// frontend to turn into a duration.
type CheckItem struct {
	Name        string `json:"name"`
	State       string `json:"state"`
	Description string `json:"description"` // workflow name, or the status context's own line
	URL         string `json:"url"`         // where to read the run; "" when gh reports none
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
}

// ghPRView mirrors the requested gh payload; statusCheckRollup is reduced to
// ChecksRollup inside parsePRDetail and never leaves this package raw.
type ghPRView struct {
	Number            int         `json:"number"`
	URL               string      `json:"url"`
	State             string      `json:"state"`
	Title             string      `json:"title"`
	Body              string      `json:"body"`
	IsDraft           bool        `json:"isDraft"`
	Mergeable         string      `json:"mergeable"`
	BaseRefName       string      `json:"baseRefName"`
	HeadRefName       string      `json:"headRefName"`
	ChangedFiles      int         `json:"changedFiles"`
	StatusCheckRollup []checkItem `json:"statusCheckRollup"`
	Commits           []ghCommit  `json:"commits"`
}

// ghCommit is one entry of gh's commits array. Authors is a list because of
// co-authored commits; the first one is the one the list names.
type ghCommit struct {
	OID             string           `json:"oid"`
	MessageHeadline string           `json:"messageHeadline"`
	MessageBody     string           `json:"messageBody"`
	CommittedDate   string           `json:"committedDate"`
	Authors         []ghCommitAuthor `json:"authors"`
}

type ghCommitAuthor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// checkItem is one statusCheckRollup entry. gh emits two shapes: a CheckRun
// (Actions/apps) carries name/status/conclusion/detailsUrl; a StatusContext
// (legacy commit statuses) carries context/state/targetUrl. checkState reads
// whichever is populated, and toCheckItems flattens the pair.
type checkItem struct {
	Status     string `json:"status"`     // CheckRun: QUEUED|IN_PROGRESS|COMPLETED|…
	Conclusion string `json:"conclusion"` // CheckRun: SUCCESS|FAILURE|NEUTRAL|SKIPPED|…
	State      string `json:"state"`      // StatusContext: SUCCESS|FAILURE|PENDING|ERROR

	Name         string `json:"name"`    // CheckRun
	Context      string `json:"context"` // StatusContext
	WorkflowName string `json:"workflowName"`
	Description  string `json:"description"` // StatusContext
	DetailsURL   string `json:"detailsUrl"`  // CheckRun
	TargetURL    string `json:"targetUrl"`   // StatusContext
	StartedAt    string `json:"startedAt"`
	CompletedAt  string `json:"completedAt"`
}

// PullRequestDetail returns the open pull request for the path's current branch
// in full, or nil when the branch has no open PR — gh reports none, or the PR is
// merged/closed, which the OPEN-only gate treats the same as the badge does. A
// real failure (gh missing, not a GitHub repo) yields an error so the panel can
// tell "no PR" apart from "could not look up".
func (s *Service) PullRequestDetail(path string) (*PRDetail, error) {
	out, err := s.gh(prReadTimeout, path, "pr", "view", "--json", prViewFields)
	if errors.Is(err, errNoPullRequest) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}
	return parsePRDetail(out)
}

// parsePRDetail decodes gh's JSON and reduces the check rollup. It returns nil
// for a non-OPEN PR — gh still reports a merged/closed branch PR, but the panel
// wants only an actionable one — matching parsePullRequest's contract.
func parsePRDetail(out []byte) (*PRDetail, error) {
	var v ghPRView
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("decode gh pr view: %w", err)
	}
	if v.Number == 0 || v.State != "OPEN" {
		return nil, nil
	}
	return &PRDetail{
		Number:       v.Number,
		URL:          v.URL,
		Title:        v.Title,
		Body:         v.Body,
		IsDraft:      v.IsDraft,
		Mergeable:    v.Mergeable,
		BaseRefName:  v.BaseRefName,
		HeadRefName:  v.HeadRefName,
		ChangedFiles: v.ChangedFiles,
		Checks:       reduceChecks(v.StatusCheckRollup),
		CheckRuns:    toCheckItems(v.StatusCheckRollup),
		Commits:      toCommits(v.Commits),
	}, nil
}

// toCommits flattens gh's commits into the list the Commits tab renders, in
// gh's own order — oldest first, the order the branch built them. Returns nil
// when gh reports none, so the tab can tell "no commits" from a list.
func toCommits(commits []ghCommit) []PRCommit {
	if len(commits) == 0 {
		return nil
	}
	out := make([]PRCommit, 0, len(commits))
	for _, c := range commits {
		author := ""
		if len(c.Authors) > 0 {
			author = firstNonEmpty(c.Authors[0].Login, c.Authors[0].Name)
		}
		out = append(out, PRCommit{
			OID:      c.OID,
			Headline: c.MessageHeadline,
			Body:     c.MessageBody,
			Author:   author,
			Date:     c.CommittedDate,
		})
	}
	return out
}

// The verdict of one check, shared by the rollup counts and the Checks tab.
const (
	checkPassed  = "passed"
	checkFailed  = "failed"
	checkPending = "pending"
)

// checkState reads gh's mixed CheckRun/StatusContext shapes into one verdict. A
// CheckRun is pending until its status is COMPLETED, then passes unless the
// conclusion is a failure; a StatusContext maps its state directly. Anything
// unrecognised is pending, so an in-flight check never reads as passed.
func checkState(it checkItem) string {
	switch {
	case it.Status != "" && it.Status != "COMPLETED":
		return checkPending
	case it.Conclusion != "":
		if isFailureConclusion(it.Conclusion) {
			return checkFailed
		}
		return checkPassed
	case it.State == "SUCCESS":
		return checkPassed
	case it.State == "FAILURE" || it.State == "ERROR":
		return checkFailed
	default:
		return checkPending
	}
}

// reduceChecks collapses the rollup into the counts the header shows.
func reduceChecks(items []checkItem) ChecksRollup {
	r := ChecksRollup{Total: len(items)}
	for _, it := range items {
		switch checkState(it) {
		case checkPassed:
			r.Passed++
		case checkFailed:
			r.Failed++
		default:
			r.Pending++
		}
	}
	return r
}

// checkOrder ranks the states for the Checks tab: what broke first, what is
// still running next, what already passed last.
var checkOrder = map[string]int{checkFailed: 0, checkPending: 1, checkPassed: 2}

// toCheckItems flattens the rollup into the list the Checks tab renders, worst
// state first and otherwise in gh's own order. Returns nil for no checks, so the
// tab can tell "none reported" from a list.
func toCheckItems(items []checkItem) []CheckItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]CheckItem, 0, len(items))
	for _, it := range items {
		out = append(out, CheckItem{
			Name:        firstNonEmpty(it.Name, it.Context),
			State:       checkState(it),
			Description: firstNonEmpty(it.WorkflowName, it.Description),
			URL:         firstNonEmpty(it.DetailsURL, it.TargetURL),
			StartedAt:   it.StartedAt,
			CompletedAt: it.CompletedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return checkOrder[out[i].State] < checkOrder[out[j].State]
	})
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func isFailureConclusion(c string) bool {
	switch c {
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE", "STALE":
		return true
	}
	return false
}

// mergeMethods maps each accepted method to its gh flag, and doubles as the
// allow-list: an unknown method is rejected before any shell-out.
var mergeMethods = map[string]string{
	"squash": "--squash",
	"merge":  "--merge",
	"rebase": "--rebase",
}

// MergePullRequest merges the path's branch PR on GitHub with the given method
// (squash|merge|rebase). A non-empty subject overrides the commit message (the
// dock's "edit message" flow); an empty subject leaves gh's default, the quick
// merge. It does not pass --delete-branch: lich's worktree removal owns that
// cleanup, and deleting a checked-out worktree's branch is trouble. gh's stderr
// is surfaced on failure (not mergeable, branch protection, gh missing).
func (s *Service) MergePullRequest(path, method, subject, body string) error {
	args, err := mergeArgs(method, subject, body)
	if err != nil {
		return err
	}
	if _, err := s.gh(prMergeTimeout, path, args...); err != nil {
		return fmt.Errorf("gh pr merge: %w", err)
	}
	return nil
}

// mergeArgs builds the gh argument list for a merge, rejecting an unknown method
// before any shell-out. The subject override applies only to squash and merge
// commits — rebase replays the branch's own commits, so gh rejects a message
// there — and an empty subject means gh's default message. body rides along with
// a subject and may itself be empty.
func mergeArgs(method, subject, body string) ([]string, error) {
	flag, ok := mergeMethods[method]
	if !ok {
		return nil, fmt.Errorf("unknown merge method %q", method)
	}
	args := []string{"pr", "merge", flag}
	if subject != "" && method != "rebase" {
		args = append(args, "--subject", subject, "--body", body)
	}
	return args, nil
}

// CreatePullRequest opens GitHub's "new pull request" page in the browser for
// the path's branch (gh pushes the branch first when it has no upstream).
// Deliberately the web flow, not an in-app form — GitHub's page already owns the
// title, body, reviewers and template. Returns once the browser is launched.
func (s *Service) CreatePullRequest(path string) error {
	if _, err := s.gh(prCreateTimeout, path, "pr", "create", "--web"); err != nil {
		return fmt.Errorf("gh pr create: %w", err)
	}
	return nil
}

// PullRequestDiff returns the unified diff of the path's branch pull request —
// every change the PR would merge into its base, as GitHub computes it — for the
// Pulls screen's "Files changed" tab. --color never keeps the output plain so
// the frontend's parseDiff reads it. An empty string with no error means the
// branch has no open PR (nothing to show).
//
// Ceiling: the whole diff is buffered here and shipped as one JSON string, so a
// monster PR costs its own size in memory on both sides. Reviewing a diff that
// large in a panel is not the workflow; stream it (or cap and mark it
// truncated) if that ever stops being true.
func (s *Service) PullRequestDiff(path string) (string, error) {
	out, err := s.gh(prReadTimeout, path, "pr", "diff", "--color", "never")
	if errors.Is(err, errNoPullRequest) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("gh pr diff: %w", err)
	}
	return string(out), nil
}

// isNoPullRequest recognises gh's "no PR for this branch" message — the one
// failure that means an empty panel rather than a real error.
func isNoPullRequest(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "no pull requests found")
}

// ghError prefers gh's own stderr (it names the cause — not mergeable, auth,
// missing repo) and falls back to the bare exec error when stderr is empty.
func ghError(stderr string, err error) string {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return msg
	}
	return err.Error()
}
