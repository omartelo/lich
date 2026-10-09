package terminal

import (
	"strconv"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// maxSubagentDepth is how many levels of subagent cards lich nests: a session
// opens cards, and so does a card it opened; a card one level further keeps its
// own subagents native. Every level multiplies the cards on screen and the
// sessions a report climbs back through, and two let a worker split its own
// task once more, which is the case nesting was asked for.
const maxSubagentDepth = 2

// subagentEnv tells a Claude Code session's lich-plugin mod how deep it runs as
// a subagent (LICH_SUBAGENT_DEPTH, 0 for a session nobody opened as one), which
// the mod's worker-answer.js reads to tell a worker, and to leave its subagents
// native (LICH_SUBAGENT_CARDS=off, read by agent-cards.js) when they do not
// become cards. The second is absent while they do, which is also what a
// session spawned by an older lich sees, so the mod's default stays the cards.
func subagentEnv(env []string, kind string, depth int, cardsOn bool) []string {
	if kind != providers.Claude {
		return env
	}
	env = append(env, "LICH_SUBAGENT_DEPTH="+strconv.Itoa(depth))
	if cardsOn {
		return env
	}
	return append(env, "LICH_SUBAGENT_CARDS=off")
}

// subagentRoute is where this spawn's briefing sends the agent to fan work out
// (relay.SubagentRoute): its own Agent tool, as cards, where a plugin release
// whose mod does that is installed; the same tool kept native in a card that
// does not open cards; and lich sessions everywhere else.
func subagentRoute(kind string, depth int, cardsOn bool) relay.SubagentRoute {
	if kind != providers.Claude || !agentplugin.ClaudeRunsSubagentCards() {
		return relay.RouteSessions
	}
	if cardsOn {
		return relay.RouteCards
	}
	if depth > 0 {
		return relay.RouteNative
	}
	return relay.RouteSessions
}

// subagentCardsOn is whether a session at depth turns its subagents into cards:
// the user's setting, a session nobody opened as a subagent, or a card above
// maxSubagentDepth whose plugin tells a worker by its depth. Under an older
// plugin a worker's own subagents stay native, since that mod would stop
// answering for a worker that opens cards.
func (s *Service) subagentCardsOn(kind string, depth int) bool {
	if !s.store.SubagentCards(kind) {
		return false
	}
	return depth == 0 || (depth < maxSubagentDepth && agentplugin.ClaudeNestsSubagentCards())
}
