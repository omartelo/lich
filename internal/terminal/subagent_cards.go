package terminal

import "github.com/omartelo/lich/internal/providers"

// subagentCardsEnv tells a Claude Code session's lich-plugin mod to leave its
// subagents native (the mod's agent-cards.js reads LICH_SUBAGENT_CARDS). The
// variable is absent while the setting is on, which is also what a session
// spawned by an older lich sees, so the mod's default stays the cards.
func subagentCardsEnv(env []string, kind string, on bool) []string {
	if kind != providers.Claude || on {
		return env
	}
	return append(env, "LICH_SUBAGENT_CARDS=off")
}
