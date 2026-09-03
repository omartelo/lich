package terminal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
)

// ompAssistantMark is what a line has to contain before it is worth
// unmarshalling: an oh-my-pi session file holds a line per tool result and per
// user turn too, and each assistant line carries the model's whole provider
// payload — tens of KB of encrypted reasoning content — so most of the file is
// bytes the cost never reads.
var ompAssistantMark = []byte(`"assistant"`)

// ompTranscriptCost adds up what an oh-my-pi conversation has cost, in USD.
//
// omp prices every assistant turn as it writes it — the provider, the model and
// a cost block sit on the message's own usage — so the total is the sum of
// those, which is the same walk omp's own status line makes (measured against
// 18.0.10, where every one of 1,395 assistant messages on this machine carried
// one). No price table is consulted: omp bills models no table here knows, and
// its zero for a free model or a subscription is an answer, not a gap.
//
// What that leaves out is what omp's own figure leaves out: a `task` sub-agent
// is not an assistant message in this file, and the cost it reports lands in
// the tool result instead. Its spend is missing from both numbers alike.
//
// The whole file is read every turn rather than resumed from an offset — a
// session file runs to a few hundred KB and the prescreen above skips most of
// it, so there is no ledger to keep in step with a rewritten file.
//
// ok is false when the file cannot be read at all. costMissUnpriced is a turn
// omp wrote without a cost of its own: the money rule the rest of the readout
// follows (usage_cost.go) says a total missing one turn is a wrong number, so
// it withholds rather than under-reports.
func ompTranscriptCost(path string) (float64, costMiss, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, costMissUnread, false
	}
	defer func() { _ = f.Close() }()

	total := 0.0
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// A trailing line with no newline is one omp is still writing; the next
			// turn reads it whole. Any other read error stopped the scan mid-file,
			// and a total that stops mid-file is a money number that is simply too
			// small.
			if err == io.EOF {
				return total, costMissNone, true
			}
			return 0, costMissUnread, false
		}
		if !bytes.Contains(line, ompAssistantMark) {
			continue
		}
		cost, ok, priced := parseOMPCostLine(line)
		if !priced {
			return 0, costMissUnpriced, false
		}
		if ok {
			total += cost
		}
	}
}

// parseOMPCostLine pulls one assistant turn's cost out of a session line. ok is
// false for every line that is not one — a user turn, a tool result, a
// malformed line, the `"assistant"` the prescreen matched inside somebody's
// prompt. priced is false only for a line that *is* an assistant turn and
// carries no total for it, which is the one absence worth withholding the
// session's number over.
func parseOMPCostLine(line []byte) (cost float64, ok, priced bool) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role  string `json:"role"`
			Usage *struct {
				Cost *struct {
					Total *float64 `json:"total"`
				} `json:"cost"`
			} `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return 0, false, true
	}
	if entry.Type != "message" || entry.Message.Role != "assistant" {
		return 0, false, true
	}
	u := entry.Message.Usage
	if u == nil || u.Cost == nil || u.Cost.Total == nil {
		return 0, false, false
	}
	return *u.Cost.Total, true, true
}
