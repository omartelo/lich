package terminal

import (
	"bufio"
	"encoding/json"
	"io"
	"os"

	"github.com/omartelo/lich/internal/pricing"
)

// rateSource prices a model's tokens. An interface so the cost scan can be
// tested against a fixed table instead of the shipped one — pricing.Table is
// the only implementation.
type rateSource interface {
	Rate(model string) (pricing.Rate, bool)
}

// costLedger is how far one transcript has been accounted for and what that
// stretch cost, mirroring the row the store keeps (see store.CostLedger). It
// travels through a scan: in as where to resume, out as where to resume next
// time.
type costLedger struct {
	offset      int64
	lastMessage string
	cost        float64
}

// costLine is the one transcript line the cost scan cares about: an assistant
// message with the four token counters a provider bills. Unlike the context
// read, a sidechain line counts — a Task sub-agent's tokens are billed to the
// same account as the turn that spawned it.
type costLine struct {
	messageID string
	model     string
	tokens    pricing.Tokens
}

// scanTranscriptCost adds up what a transcript has cost, resuming from where
// the last scan stopped so a turn re-reads only what was appended since. ok is
// false when the file cannot be read at all.
//
// complete reports that the scan reached the end of the file. It is false when
// a line names a model nothing can price yet: the scan stops there rather than
// skipping the line, because a total missing one turn is a wrong number, and a
// wrong number about money is worse than none. The lookup that missed schedules
// the price refresh that may unblock it, so the next turn resumes from the same
// line — this heals itself or stays absent, and never guesses.
func scanTranscriptCost(path string, from costLedger, rates rateSource) (costLedger, bool, bool) {
	f, err := os.Open(path)
	if err != nil {
		return from, false, false
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return from, false, false
	}
	// A transcript shorter than what was already counted is not the file that
	// was counted: it was replaced or truncated, so the row it backs is rebuilt
	// from zero rather than continued into content that is no longer there.
	if info.Size() < from.offset {
		from = costLedger{}
	}
	if _, err := f.Seek(from.offset, io.SeekStart); err != nil {
		return from, false, false
	}
	ledger, complete := accumulate(bufio.NewReader(f), from, rates)
	return ledger, complete, true
}

// accumulate walks complete lines from r, folding each priced assistant message
// into the ledger. A trailing line with no newline is left unconsumed: the
// provider is still writing it, and the next scan reads it whole.
//
// Only io.EOF means the scan reached the end. Any other read error stopped it
// somewhere in the middle, and a total that stops in the middle of a transcript
// is a money number that is simply too small — the caller shows nothing and the
// next turn resumes from the same offset.
func accumulate(r *bufio.Reader, ledger costLedger, rates rateSource) (costLedger, bool) {
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return ledger, err == io.EOF
		}
		entry, ok := parseCostLine(line)
		// An id is what makes a repeat recognisable; a line carrying none is
		// counted on its own merits rather than folded into the last message.
		repeat := entry.messageID != "" && entry.messageID == ledger.lastMessage
		if !ok || repeat {
			// Not billable, or the same assistant message written across several
			// lines — each repeats that message's cumulative usage, so counting
			// the repeat would bill the turn twice.
			ledger.offset += int64(len(line))
			continue
		}
		rate, priced := rates.Rate(entry.model)
		if !priced {
			return ledger, false
		}
		ledger.cost += rate.Cost(entry.tokens)
		ledger.lastMessage = entry.messageID
		ledger.offset += int64(len(line))
	}
}

// parseCostLine pulls the billable content out of a transcript line. false for
// anything that is not an assistant message with tokens on it — a user turn, a
// malformed line, or one of the synthetic assistant entries a provider writes
// for its own errors, which report no tokens and so name no model to price.
func parseCostLine(line []byte) (costLine, bool) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage *struct {
				Input       int `json:"input_tokens"`
				Output      int `json:"output_tokens"`
				CacheRead   int `json:"cache_read_input_tokens"`
				CacheCreate int `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return costLine{}, false
	}
	if entry.Type != "assistant" || entry.Message.Usage == nil {
		return costLine{}, false
	}
	u := entry.Message.Usage
	tokens := pricing.Tokens{
		Input:      u.Input,
		Output:     u.Output,
		CacheRead:  u.CacheRead,
		CacheWrite: u.CacheCreate,
	}
	if tokens == (pricing.Tokens{}) {
		return costLine{}, false
	}
	return costLine{messageID: entry.Message.ID, model: entry.Message.Model, tokens: tokens}, true
}
