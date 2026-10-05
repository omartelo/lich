package terminal

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// The lich-plugin mod reads LICH_SUBAGENT_CARDS to leave a Claude Code
// session's subagents native; lich sets it only when the user turned the
// setting off, so a session spawned with it on carries no variable at all.
func TestSubagentCardsEnv(t *testing.T) {
	base := []string{"A=1"}
	cases := []struct {
		name string
		kind string
		on   bool
		want []string
	}{
		{"claude with cards on", providers.Claude, true, []string{"A=1"}},
		{"claude with cards off", providers.Claude, false, []string{"A=1", "LICH_SUBAGENT_CARDS=off"}},
		{"another provider with the setting off", providers.Codex, false, []string{"A=1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := subagentCardsEnv(slices.Clone(base), c.kind, c.on)
			if !slices.Equal(got, c.want) {
				t.Errorf("subagentCardsEnv = %v, want %v", got, c.want)
			}
		})
	}
}
