package terminal

import (
	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/providers"
)

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

// subagentCardsOn is whether a session's subagents become cards: the user's
// setting, except in a session that is itself a subagent's card
// (store.SessionSubagent), whose own subagents stay native so a worker sharing
// its caller's checkout never opens cards of its own.
// briefsAgentCards is whether this spawn's briefing sends the agent to its own
// Agent tool: a Claude Code session whose subagents become cards, with a plugin
// release whose mod does that installed.
func (s *Service) briefsAgentCards(id, kind string) bool {
	return kind == providers.Claude && s.subagentCardsOn(id, kind) && agentplugin.ClaudeRunsSubagentCards()
}

func (s *Service) subagentCardsOn(id, kind string) bool {
	return s.store.SubagentCards(kind) && !s.store.SessionSubagent(id)
}
