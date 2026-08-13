// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"bufio"
	"errors"
	"io"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// haltingReader serves body and then fails, the way a transcript on a
// disappearing mount or a file replaced mid-read does: some lines arrive, the
// rest never will.
type haltingReader struct {
	body []byte
	err  error
}

func (r *haltingReader) Read(p []byte) (int, error) {
	if len(r.body) == 0 {
		return 0, r.err
	}
	n := copy(p, r.body)
	r.body = r.body[n:]
	return n, nil
}

// A read that fails halfway has not reached the end of the transcript, and the
// lines it did reach add up to a total that is simply too small. Reported as
// complete it becomes a money number the user reads as the session's cost.
func TestAFailedReadIsNotACompleteScan(t *testing.T) {
	body := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n"
	r := &haltingReader{body: []byte(body), err: errors.New("input/output error")}

	ledger, complete := accumulate(bufio.NewReader(r), costLedger{}, testRate)

	if complete {
		t.Error("a scan stopped by a read error reported itself complete")
	}
	if ledger.cost == 0 {
		t.Error("the lines that were read were dropped, want them kept for the next resume")
	}
}

// The end of the file is the one error that means the scan is done — the same
// reader, stopping the same way, but with io.EOF.
func TestReachingTheEndIsACompleteScan(t *testing.T) {
	body := costLineJSON("m1", modelOpus, 100, 0, 0, 0) + "\n"
	r := &haltingReader{body: []byte(body), err: io.EOF}

	if _, complete := accumulate(bufio.NewReader(r), costLedger{}, testRate); !complete {
		t.Error("a scan that reached the end of the file reported itself incomplete")
	}
}

// Every store failure on the cost path is a miss, never a number: the ledger
// that says where to resume, the write that moves it, and the total itself all
// speak for the same money, and a total read against a ledger that failed is a
// wrong one. Each is warned once and the readout keeps its last value.
func TestAStoreFailureShowsNoCost(t *testing.T) {
	failure := errors.New("the database is gone")
	tests := []struct {
		name  string
		store stubBins
	}{
		{"the ledger cannot be read", stubBins{ledgerErr: failure}},
		{"the ledger cannot be written", stubBins{saveLedgerErr: failure}},
		{"the total cannot be summed", stubBins{sessionCostErr: failure}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 10, 0, 0)+"\n")
			store := tc.store
			store.providerSession = "uuid-cost"
			store.costOn = true
			store.ledgers = make(map[string]stubLedger)
			svc := New(store, nil, events.New())
			svc.prices = testRate

			if cost, ok := svc.sessionCost("s1", "uuid-cost"); ok {
				t.Errorf("sessionCost = %v, want a miss", cost)
			}
			// The context readout beside it is unaffected: only the money is
			// missing, and the card still reports its window.
			got, ok := svc.sessionUsage("s1")
			if !ok {
				t.Fatal("sessionUsage: want ok — the context readout does not ride on the store")
			}
			if got.CostUSD != nil {
				t.Errorf("CostUSD = %v, want absent", *got.CostUSD)
			}
		})
	}
}
