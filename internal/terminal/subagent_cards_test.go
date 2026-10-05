package terminal

import (
	"slices"
	"strings"
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

// A Claude Code spawn whose subagents become lich cards is briefed to fan out
// through its own Agent tool; every other spawn keeps the line against it.
func TestBriefingFollowsSubagentCards(t *testing.T) {
	cards := providerArgs(providers.Claude, "", "", "", "", "/usr/bin/lich", "", false, false, false, true)
	plain := providerArgs(providers.Claude, "", "", "", "", "/usr/bin/lich", "", false, false, false, false)
	briefing := func(args []string) string {
		at := slices.Index(args, "--append-system-prompt")
		if at < 0 || at+1 >= len(args) {
			t.Fatalf("no briefing in %v", args)
		}
		return args[at+1]
	}
	if !strings.Contains(briefing(cards), "Agent tool") {
		t.Errorf("a card spawn is not pointed at the Agent tool:\n%s", briefing(cards))
	}
	if strings.Contains(briefing(plain), "Agent tool") {
		t.Errorf("a spawn without cards is pointed at the Agent tool:\n%s", briefing(plain))
	}
}
