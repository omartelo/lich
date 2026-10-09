//go:build !windows

package terminal

import (
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// Unix-only with the stubBins it reads, which live in the PTY suite.

// A worker opens cards one level down, and only under a plugin that tells a
// worker by its depth; a card at maxSubagentDepth keeps its own subagents
// native whatever the setting says.
func TestSubagentCardsOnFollowsTheDepth(t *testing.T) {
	cases := []struct {
		name   string
		bins   stubBins
		plugin string
		wantOn bool
	}{
		{"setting on", stubBins{}, "0.19.0", true},
		{"setting off", stubBins{subagentCardsOff: true}, "0.19.0", false},
		{"a worker", stubBins{subagentDepth: 1}, "0.19.0", true},
		{"a worker under an older plugin", stubBins{subagentDepth: 1}, "0.18.4", false},
		{"a worker with the setting off", stubBins{subagentDepth: 1, subagentCardsOff: true}, "0.19.0", false},
		{"a worker's worker", stubBins{subagentDepth: 2}, "0.19.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withClaudePlugin(t, c.plugin)
			svc := New(c.bins, nil, events.New())
			depth := c.bins.SessionSubagentDepth("s1")
			if got := svc.subagentCardsOn(providers.Claude, depth); got != c.wantOn {
				t.Errorf("subagentCardsOn = %v, want %v", got, c.wantOn)
			}
		})
	}
}

// A fork of a worker's conversation carries the worker's depth onto its own
// row before it spawns, so the copy opens cards exactly as the worker would.
func TestForkOfASubagentKeepsItsDepth(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	store := stubBins{
		bin:               stayAliveBin(t),
		subagentParents:   map[string]int{"conv-worker": maxSubagentDepth},
		inheritedSubagent: map[string]int{},
	}
	svc := New(store, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s2") })

	if err := svc.Start("s2", "p1", t.TempDir(), providers.Claude, "conv-worker", "", true, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	if got := store.inheritedSubagent["s2"]; got != maxSubagentDepth {
		t.Errorf("the fork's row is %d deep, want the worker's %d", got, maxSubagentDepth)
	}
	if svc.subagentCardsOn(providers.Claude, store.SessionSubagentDepth("s2")) {
		t.Error("the fork of a worker at the limit opens cards, want its subagents native")
	}
}
