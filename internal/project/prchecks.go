package project

import "sort"

// gh reports a pull request's CI as a mixed array of two unrelated shapes. This
// file is the whole translation from that array into the two things the screen
// wants — a header count and a ranked list — and nothing else in the package
// needs to know either shape exists.

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

func isFailureConclusion(c string) bool {
	switch c {
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE", "STALE":
		return true
	}
	return false
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
