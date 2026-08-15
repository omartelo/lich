// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/pricing"
)

// fixedRates prices only what it was given, so a test states its own economy
// and an unpriced model is a deliberate case rather than a table lookup.
type fixedRates map[string]pricing.Rate

func (f fixedRates) Rate(model string) (pricing.Rate, bool) {
	rate, ok := f[model]
	return rate, ok
}

// testRate is round enough to read totals off by eye: 1 unit of anything is
// $0.001, except output at $0.01 and the hour-long cache write at $0.002 — the
// two cache writes are deliberately different, so a total billed at one rate for
// both is visible in the number.
var testRate = fixedRates{
	modelOpus: {Input: 0.001, Output: 0.01, CacheRead: 0.001, CacheWrite: 0.001, CacheWrite1h: 0.002},
}

// costLine renders an assistant transcript line with an explicit message id and
// the four counters the bill is built from.
func costLineJSON(id, model string, input, output, cacheRead, cacheCreate int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","model":"` + model +
		`","usage":{"input_tokens":` + itoa(input) +
		`,"output_tokens":` + itoa(output) +
		`,"cache_read_input_tokens":` + itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + itoa(cacheCreate) + `}}}`
}

// cacheLineJSON renders an assistant line whose whole bill is one cache write,
// split the way the provider reports it: a total, and the share of that total
// held for an hour.
func cacheLineJSON(id, model string, cacheCreate, hour int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","model":"` + model +
		`","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0` +
		`,"cache_creation_input_tokens":` + itoa(cacheCreate) +
		`,"cache_creation":{"ephemeral_5m_input_tokens":` + itoa(cacheCreate-hour) +
		`,"ephemeral_1h_input_tokens":` + itoa(hour) + `}}}}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return digits
}

// writeFile plants a transcript at a throwaway path and returns it.
func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCostSumsEveryAssistantLine is the traversal the readout needs and the
// context gauge does not: the whole file, not its tail, and output tokens
// included — they are the expensive half of a bill.
func TestCostSumsEveryAssistantLine(t *testing.T) {
	path := writeFile(t,
		costLineJSON("m1", modelOpus, 100, 10, 0, 0)+"\n"+
			`{"type":"user","message":{"content":"hi"}}`+"\n"+
			costLineJSON("m2", modelOpus, 200, 20, 0, 0)+"\n")

	ledger, complete, ok := scanTranscriptCost(path, costLedger{}, testRate)

	if !ok || !complete {
		t.Fatalf("scan: ok=%v complete=%v, want both true", ok, complete)
	}
	want := 300*0.001 + 30*0.01
	if !nearly(ledger.cost, want) {
		t.Errorf("cost = %v, want %v", ledger.cost, want)
	}
}

// TestCostResumesWhereItStopped proves the scan is incremental: it re-reads
// only what was appended. emitUsage runs on every non-idle hook, so recounting
// the file each time would make a long session's readout cost more than the
// turn it reports on.
func TestCostResumesWhereItStopped(t *testing.T) {
	first := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n"
	path := writeFile(t, first)

	ledger, _, _ := scanTranscriptCost(path, costLedger{}, testRate)
	if ledger.offset != int64(len(first)) {
		t.Fatalf("offset = %d, want %d (the whole first line)", ledger.offset, len(first))
	}

	appended := costLineJSON("m2", modelOpus, 100, 0, 0, 0) + "\n"
	if err := os.WriteFile(path, []byte(first+appended), 0o644); err != nil {
		t.Fatal(err)
	}
	next, _, _ := scanTranscriptCost(path, ledger, testRate)

	if !nearly(next.cost, 0.2) {
		t.Errorf("cost = %v, want 0.2 — the resumed scan must add, not recount", next.cost)
	}
}

// TestAPartialLineIsLeftForTheNextScan pins the mid-write case: the provider
// appends while lich reads, so a tail with no newline is content that is not
// there yet. Counting it would bill a half-written number.
func TestAPartialLineIsLeftForTheNextScan(t *testing.T) {
	whole := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n"
	path := writeFile(t, whole+`{"type":"assistant","message":{"id":"m2"`)

	ledger, complete, ok := scanTranscriptCost(path, costLedger{}, testRate)

	if !ok || !complete {
		t.Fatalf("scan: ok=%v complete=%v, want both true", ok, complete)
	}
	if ledger.offset != int64(len(whole)) {
		t.Errorf("offset = %d, want %d — the partial line must stay unread", ledger.offset, len(whole))
	}
	if !nearly(ledger.cost, 0.1) {
		t.Errorf("cost = %v, want only the complete line's 0.1", ledger.cost)
	}
}

// TestOneMessageWrittenTwiceIsBilledOnce pins the dedup. A provider may write
// one assistant message across several lines, each repeating that message's
// cumulative usage; counting the repeat doubles the turn.
func TestOneMessageWrittenTwiceIsBilledOnce(t *testing.T) {
	line := costLineJSON("m1", modelOpus, 100, 0, 0, 0)
	path := writeFile(t, line+"\n"+line+"\n")

	ledger, _, _ := scanTranscriptCost(path, costLedger{}, testRate)

	if !nearly(ledger.cost, 0.1) {
		t.Errorf("cost = %v, want 0.1 — the repeated message must be billed once", ledger.cost)
	}
}

// TestTheDedupSurvivesAScanBoundary is the same contract across two reads: the
// repeat can land in the next scan, so the last billed message id is part of
// what the ledger remembers.
func TestTheDedupSurvivesAScanBoundary(t *testing.T) {
	line := costLineJSON("m1", modelOpus, 100, 0, 0, 0)
	path := writeFile(t, line+"\n")

	ledger, _, _ := scanTranscriptCost(path, costLedger{}, testRate)
	if err := os.WriteFile(path, []byte(line+"\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, _, _ := scanTranscriptCost(path, ledger, testRate)

	if !nearly(next.cost, 0.1) {
		t.Errorf("cost = %v, want 0.1 — the repeat crossed a scan and was billed again", next.cost)
	}
}

// TestAnUnpricedModelStopsTheCount is the money invariant. A line nothing can
// price is not skipped: skipping it would report a total that silently misses a
// turn. The scan stops there, the readout goes absent, and the same line is
// retried once a price for it lands.
func TestAnUnpricedModelStopsTheCount(t *testing.T) {
	path := writeFile(t,
		costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n"+
			costLineJSON("m2", "model-from-the-future", 900, 0, 0, 0)+"\n"+
			costLineJSON("m3", modelOpus, 100, 0, 0, 0)+"\n")

	ledger, complete, ok := scanTranscriptCost(path, costLedger{}, testRate)

	if !ok {
		t.Fatal("scan: want ok")
	}
	if complete {
		t.Error("complete = true, want false — an unpriced line must withhold the total")
	}
	if !nearly(ledger.cost, 0.1) {
		t.Errorf("cost = %v, want only the priced line before the stop", ledger.cost)
	}

	// The price lands (the miss scheduled the refresh) and the same line is
	// picked up from where the scan stopped.
	taught := fixedRates{modelOpus: testRate[modelOpus], "model-from-the-future": {Input: 0.001}}
	next, complete, _ := scanTranscriptCost(path, ledger, taught)
	if !complete {
		t.Error("complete = false after the price landed, want true")
	}
	if !nearly(next.cost, 0.1+0.9+0.1) {
		t.Errorf("cost = %v, want every line counted once the model was priced", next.cost)
	}
}

// TestASyntheticLineIsNotAModelToPrice proves the provider's own error entries
// — no tokens, and a placeholder where the model id goes — neither cost
// anything nor block the count behind a model that will never have a price.
func TestASyntheticLineIsNotAModelToPrice(t *testing.T) {
	path := writeFile(t,
		costLineJSON("m1", "<synthetic>", 0, 0, 0, 0)+"\n"+
			costLineJSON("m2", modelOpus, 100, 0, 0, 0)+"\n")

	ledger, complete, ok := scanTranscriptCost(path, costLedger{}, testRate)

	if !ok || !complete {
		t.Fatalf("scan: ok=%v complete=%v, want both true", ok, complete)
	}
	if !nearly(ledger.cost, 0.1) {
		t.Errorf("cost = %v, want 0.1 — the synthetic line must be passed over", ledger.cost)
	}
}

// TestACacheWriteIsPricedByHowLongItIsKept is the split the bill makes and the
// counter hides: cache_creation_input_tokens is the whole write, and the hour
// share of it costs more than the five-minute one. Billing the total at a single
// rate reads a session as cheaper than it was. The wants are written out, not
// rebuilt from testRate, so a rate applied to the wrong counter fails here.
func TestACacheWriteIsPricedByHowLongItIsKept(t *testing.T) {
	tests := []struct {
		name        string
		cacheCreate int
		hour        int
		want        float64
	}{
		{"every token held for an hour", 1000, 1000, 2.0},
		{"a write split across both lifetimes", 1000, 600, 1.6},
		{"nothing held for an hour", 1000, 0, 1.0},
		// A transcript from before the provider reported the split says nothing
		// about lifetimes, and the whole write reads as the five-minute one.
		{"a transcript with no split at all", 1000, -1, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := cacheLineJSON("m1", modelOpus, tc.cacheCreate, tc.hour)
			if tc.hour < 0 {
				line = costLineJSON("m1", modelOpus, 0, 0, 0, tc.cacheCreate)
			}
			ledger, complete, ok := scanTranscriptCost(writeFile(t, line+"\n"), costLedger{}, testRate)

			if !ok || !complete {
				t.Fatalf("scan: ok=%v complete=%v, want both true", ok, complete)
			}
			if !nearly(ledger.cost, tc.want) {
				t.Errorf("cost = %v, want %v", ledger.cost, tc.want)
			}
		})
	}
}

// TestAnUnpricedCacheLifetimeStopsTheCount carries the unpriced-model rule down
// to the counter: a model whose hour-long write has no published price cannot be
// billed at the five-minute one, because that total is wrong in the direction
// that flatters the bill.
func TestAnUnpricedCacheLifetimeStopsTheCount(t *testing.T) {
	fiveMinuteOnly := fixedRates{modelOpus: {Input: 0.001, Output: 0.01, CacheRead: 0.001, CacheWrite: 0.001}}
	path := writeFile(t, cacheLineJSON("m1", modelOpus, 1000, 1000)+"\n")

	ledger, complete, ok := scanTranscriptCost(path, costLedger{}, fiveMinuteOnly)

	if !ok {
		t.Fatal("scan: want ok")
	}
	if complete {
		t.Error("complete = true, want false — an hour-long write with no price must withhold the total")
	}
	if ledger.cost != 0 {
		t.Errorf("cost = %v, want nothing counted past the line it cannot price", ledger.cost)
	}
}

// TestASubAgentsTokensAreBilled is where cost parts ways with the context
// gauge: a sub-agent's sidechain lines are excluded from the window the user
// sees, but they are billed to the same account. This is the shape a Claude
// conversation used to have — the lines inline in the parent — and it is still
// counted; where the same tokens live now is
// TestASubAgentTranscriptIsBilledIntoTheSession.
func TestASubAgentsTokensAreBilled(t *testing.T) {
	sidechain := `{"type":"assistant","isSidechain":true,"message":{"id":"m2","model":"` + modelOpus +
		`","usage":{"input_tokens":100,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`
	path := writeFile(t, costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n"+sidechain+"\n")

	ledger, _, _ := scanTranscriptCost(path, costLedger{}, testRate)

	if !nearly(ledger.cost, 0.2) {
		t.Errorf("cost = %v, want 0.2 — a sub-agent's tokens are billed too", ledger.cost)
	}
}

// TestAReplacedTranscriptIsRecounted pins the file-shrank case: a transcript
// shorter than what was already counted is not the file that was counted, so
// its row is rebuilt rather than resumed into content that is gone.
func TestAReplacedTranscriptIsRecounted(t *testing.T) {
	path := writeFile(t, costLineJSON("m1", modelOpus, 500, 0, 0, 0)+"\n")
	ledger, _, _ := scanTranscriptCost(path, costLedger{}, testRate)

	shorter := costLineJSON("m9", modelOpus, 10, 0, 0, 0) + "\n"
	if err := os.WriteFile(path, []byte(shorter), 0o644); err != nil {
		t.Fatal(err)
	}
	next, _, ok := scanTranscriptCost(path, ledger, testRate)

	if !ok {
		t.Fatal("scan: want ok")
	}
	if !nearly(next.cost, 0.01) {
		t.Errorf("cost = %v, want 0.01 — the replaced file must be counted from zero", next.cost)
	}
}

// TestAnUnreadableTranscriptIsAMiss keeps the readout on the same "degrade,
// never break" contract as the context gauge.
func TestAnUnreadableTranscriptIsAMiss(t *testing.T) {
	if _, _, ok := scanTranscriptCost(filepath.Join(t.TempDir(), "nope.jsonl"), costLedger{}, testRate); ok {
		t.Error("scan: want a miss for a transcript that does not exist")
	}
}

// TestSessionCostRidesOnTheUsageEvent proves the wiring end to end: with the
// setting on, the event carries what the session has cost.
func TestSessionCostRidesOnTheUsageEvent(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 10, 0, 0)+"\n")
	svc := New(newCostStore("uuid-cost"), nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok")
	}
	if got.CostUSD == nil {
		t.Fatal("CostUSD is absent, want the session's cost")
	}
	if !nearly(*got.CostUSD, 0.1+0.1) {
		t.Errorf("CostUSD = %v, want 0.2", *got.CostUSD)
	}
}

// TestTheCostIsAbsentWhenTheFlagIsOff is the feature's premise: on a
// subscription the number is noise, so it is not reported at all — not zero,
// not stale. The context readout beside it is unaffected.
func TestTheCostIsAbsentWhenTheFlagIsOff(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 10, 0, 0)+"\n")
	svc := New(stubBins{providerSession: "uuid-cost"}, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok — the context readout still reports")
	}
	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent while the setting is off", *got.CostUSD)
	}
}

// TestTheCostIsAbsentWhileAModelIsUnpriced carries the scan's invariant out to
// the event: no number is shown until every counted turn has a price.
func TestTheCostIsAbsentWhileAModelIsUnpriced(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", "model-from-the-future", 100, 0, 0, 0)+"\n")
	svc := New(newCostStore("uuid-cost"), nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok")
	}
	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent while a model has no price", *got.CostUSD)
	}
}

// TestASubAgentTranscriptIsBilledIntoTheSession is the file the conversation
// does not contain: a sub-agent runs its own transcript, and its tokens are
// billed to the account that spawned it. Counting only the parent bills a
// session for delegating the work and not for doing it.
func TestASubAgentTranscriptIsBilledIntoTheSession(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	writeSubagentTranscript(t, "uuid-cost", "agent-one", costLineJSON("m2", modelOpus, 300, 0, 0, 0)+"\n")
	writeSubagentTranscript(t, "uuid-cost", "agent-two", costLineJSON("m3", modelOpus, 600, 0, 0, 0)+"\n")
	svc := New(newCostStore("uuid-cost"), nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok")
	}
	if got.CostUSD == nil || !nearly(*got.CostUSD, 1.0) {
		t.Errorf("CostUSD = %v, want 1.0 — both sub-agents count into the session", got.CostUSD)
	}
}

// TestAnUnpricedSubAgentWithholdsTheTotal carries the money rule across files: a
// session's cost is only as reportable as the least readable transcript behind
// it, and a total missing one sub-agent is wrong in the flattering direction.
func TestAnUnpricedSubAgentWithholdsTheTotal(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	writeSubagentTranscript(t, "uuid-cost", "agent-one",
		costLineJSON("m2", "model-from-the-future", 300, 0, 0, 0)+"\n")
	svc := New(newCostStore("uuid-cost"), nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok — the context readout is unaffected")
	}
	if got.CostUSD != nil {
		t.Errorf("CostUSD = %v, want absent while a sub-agent has an unpriced model", *got.CostUSD)
	}
}

// TestCostAccumulatesAcrossConversations is the `/clear` case: a new
// conversation under the same session gets its own transcript, and the session
// keeps what the previous one cost.
func TestCostAccumulatesAcrossConversations(t *testing.T) {
	store := newCostStore("uuid-first")
	writeTranscript(t, "uuid-first", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	svc := New(store, nil, events.New())
	svc.prices = testRate
	if _, ok := svc.sessionUsage("s1"); !ok {
		t.Fatal("sessionUsage: want ok on the first conversation")
	}

	// /clear: same lich session, a fresh provider conversation and transcript.
	writeTranscript(t, "uuid-second", costLineJSON("m2", modelOpus, 300, 0, 0, 0)+"\n")
	store.providerSession = "uuid-second"
	svc = New(store, nil, events.New())
	svc.prices = testRate

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok after the clear")
	}
	if got.CostUSD == nil || !nearly(*got.CostUSD, 0.4) {
		t.Errorf("CostUSD = %v, want 0.4 — the cleared conversation still counts", got.CostUSD)
	}
}

// nearly compares two dollar amounts built from float multiplications, where
// the last bits of a sum are not worth asserting on.
func nearly(got, want float64) bool {
	diff := got - want
	return diff < 1e-9 && diff > -1e-9
}
