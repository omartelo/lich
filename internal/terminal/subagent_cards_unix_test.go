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
