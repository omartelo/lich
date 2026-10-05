//go:build !windows

package terminal

import (
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// Unix-only with the stubBins it reads, which live in the PTY suite.

// A session opened as a subagent keeps its own subagents native whatever the
// setting says: a card does not open cards.
func TestASubagentSessionNeverOpensCards(t *testing.T) {
	cases := []struct {
		name   string
		bins   stubBins
		wantOn bool
	}{
		{"setting on", stubBins{}, true},
		{"setting off", stubBins{subagentCardsOff: true}, false},
		{"opened as a subagent", stubBins{subagentSession: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := New(c.bins, nil, events.New())
			if got := svc.subagentCardsOn("s1", providers.Claude); got != c.wantOn {
				t.Errorf("subagentCardsOn = %v, want %v", got, c.wantOn)
			}
		})
	}
}

// A fork of a worker's conversation carries the subagent mark onto its own row
// before it spawns, so the copy never opens cards either.
func TestForkOfASubagentKeepsItsSubagentsNative(t *testing.T) {
	store := stubBins{
		bin:               stayAliveBin(t),
		subagentParents:   map[string]bool{"conv-worker": true},
		inheritedSubagent: map[string]bool{},
	}
	svc := New(store, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s2") })

	if err := svc.Start("s2", "p1", t.TempDir(), providers.Claude, "conv-worker", "", true, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	if !store.inheritedSubagent["s2"] {
		t.Error("the fork's row was not marked a subagent")
	}
	if svc.subagentCardsOn("s2", providers.Claude) {
		t.Error("the fork of a worker opens cards, want its subagents native")
	}
}
