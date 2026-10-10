package relay

import (
	"fmt"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/prompt"
)

// The message lich types at the target's prompt, and the keystrokes that put it
// there. Everything the receiving agent knows about this feature it reads in
// this text: the words are the whole protocol, which is what lets a provider
// lich has never heard of answer a relayed request.

// SpawnBriefing is what a provider is told about lich when lich starts it: two
// sentences appended to its system prompt, for the providers whose command line
// accepts one (internal/terminal, briefingFlags).
//
// It exists because the agent's own harness offers subagents of its own, and
// those are described to it at length, in its system prompt, from its first
// turn. lich's tools are described in a server block far below that — so an
// agent asked to "fan this out across worktrees" reaches for the subagent it was
// told about, and the user watches work they meant to supervise disappear into a
// process they cannot open, on a checkout that does not exist. Reported twice by
// the same user before this text existed.
//
// It draws the line rather than forbidding anything: a subagent is still the
// right tool for a throwaway read. What it fixes is the one case the agent
// cannot get right on its own, because nothing in its prompt says lich sessions
// are visible and steerable and subagents are not.
//
// lang is the session's prompt language; every locale keeps to the quoting
// rule below (see prompt.Catalog).
//
// hasTools is whether this provider was handed lich's MCP server at spawn. The
// command line works everywhere and is named for the ones that were not, on the
// same rule replyInstruction follows: naming a tool a session does not have is
// worse than naming the command.
//
// The example spells its placeholders in capitals and quotes with ' rather than
// the usual <angle brackets> and ": this string is passed as one argv entry, and
// a provider shipped as a .cmd is spawned through `cmd.exe /c`
// (internal/terminal, wrapArgv). Windows escapes a double quote as \" for
// CommandLineToArgvW, cmd.exe does not read that as an escape, and the < and >
// left outside quotes by it are redirection.
//
// route is where this spawn's own subagents run (SubagentRoute), which decides
// what the agent is sent to for fanning work out.
func SpawnBriefing(lang prompt.Lang, hasTools bool, route SubagentRoute) string {
	text := prompt.For(lang)
	command := text.BriefingCommandCLI
	if hasTools {
		command = text.BriefingCommandTools
	}
	switch route {
	case RouteCards:
		return text.BriefingIntro + text.BriefingCards + command
	case RouteNative:
		return text.BriefingIntro + text.BriefingNative
	default:
		return text.BriefingIntro + text.BriefingSessions + command
	}
}

// SubagentRoute is where a spawned session's own subagents run, as far as its
// briefing is concerned.
type SubagentRoute int

const (
	// RouteSessions is every session whose subagents stay inside its harness and
	// that may open lich sessions to fan out.
	RouteSessions SubagentRoute = iota
	// RouteCards is a Claude Code session whose lich-plugin mod runs a
	// general-purpose subagent as one of those cards. There the agent's own
	// Agent tool is the better route (in the background, its report back on its
	// own, stopped with TaskStop), so the briefing sends it there and keeps
	// lich's own route for what a subagent cannot be.
	RouteCards
	// RouteNative is a Claude Code card whose own subagents stay native, because
	// it runs as deep as lich nests cards. lich's server instructions still say
	// a subagent becomes a card in a session with the plugin, so the briefing
	// says it does not here, and keeps the fan-out under this card rather than
	// sending it to new sessions, which would nest cards past the limit by
	// another door.
	RouteNative
)

// compose is the message typed at the target's prompt. It names where the
// request came from so the receiving agent knows this did not come from the
// person in front of it, and carries the exact command that sends an answer
// home — the reply path only exists because this text describes it, so the
// agent needs no prior knowledge of the feature.
func compose(lang prompt.Lang, sender, ticketID, task string, hasTools bool) string {
	return fmt.Sprintf(
		prompt.For(lang).RelayMessage,
		origin(lang, sender), task, replyInstruction(lang, hasTools, ticketID),
	)
}

// composeForWorker is the task handed to a worker whose mod answers for it
// (docs/hooks/mod-answer.md): the task under one line naming who asked, the way
// a native subagent is handed its prompt. There is no ticket and no reply
// command, because the worker's last message is its answer.
func composeForWorker(lang prompt.Lang, sender, task string) string {
	return fmt.Sprintf(prompt.For(lang).WorkerTask, sender, task)
}

// replyInstruction tells the receiving agent how to answer. Every agent has a
// shell, so the command is always named; a provider lich registers its MCP
// server with is offered the tool first, because a session that withholds shell
// access would otherwise have no way to answer at all.
//
// The exclusivity is load-bearing, not emphasis. A Claude Code session has a
// peer channel of its own and this message names another session, so answering
// through that channel is the obvious move — and it reaches a sender that is
// blocked on a ticket instead. That happened on the first real run: the target
// replied over its own socket and the errand timed out with the answer already
// written. The ticket is the only route home, and the message has to say so.
func replyInstruction(lang prompt.Lang, hasTools bool, ticketID string) string {
	text := prompt.For(lang)
	command := fmt.Sprintf(text.ReplyCommand, ticketID)
	route := fmt.Sprintf(text.ReplyRouteCLI, command)
	if hasTools {
		route = fmt.Sprintf(text.ReplyRouteTools, ToolReply, ticketID, command)
	}
	return route + "\n\n" + text.ReplyOnlyWayBack
}

// pickTicketNudge is what a worker is told at its own prompt after a turn that
// ended with none of its errands answered. Those errands are over by the time it
// reads this — every one the turn could have been went home unanswered, which is
// the same true thing about each of them — so they are named as history and not
// as somewhere to reply: a ticket the relay has closed answers "unknown ticket",
// and a note that invites that is worse than no note. What it asks for is the
// next answer, which is the one that can still name its ticket.
func pickTicketNudge(lang prompt.Lang, count int, errands string) string {
	return fmt.Sprintf(prompt.For(lang).PickTicketNudge, count, errands)
}

// nudgeNotice is the one line typed at a sender's prompt when results are
// waiting and nobody is holding the line for them. It replaces typing the
// results themselves: N results landing as N prompt submissions each restart
// the sender's turn and leave the full text sitting in its context window,
// while one short line lets the agent drain everything in a single tool call.
// count and targets cover everything waiting, not only what this nudge is the
// first to mention — the reader acts on the total.
func nudgeNotice(lang prompt.Lang, count int, targets []string, hasTools bool) string {
	text := prompt.For(lang)
	what := fmt.Sprintf(text.Pick(count, text.NudgeResultsReady), count, QuotedList(targets))
	route := text.NudgeRouteCLI
	if hasTools {
		route = fmt.Sprintf(text.NudgeRouteTools, ToolCollect)
	}
	return fmt.Sprintf(text.NudgeNotice, what, route)
}

// QuotedList words a list of names (session labels, folders) for a message:
// each one quoted, so a name with a comma or a trailing space still reads as
// one.
func QuotedList(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return strings.Join(quoted, ", ")
}

// origin describes the sender in the message's first line. An empty sender is
// the lich CLI run outside any session — a script, a scheduled job, the user's
// own shell — which is a different thing to be told than "another agent".
func origin(lang prompt.Lang, sender string) string {
	text := prompt.For(lang)
	if sender == "" {
		return text.OriginCLI
	}
	return fmt.Sprintf(text.OriginSession, sender)
}

// paste wraps text in bracketed paste, which is how a multi-line message
// reaches a TUI prompt as one prompt instead of as one submission per newline.
// Every provider lich spawns runs a TUI that enables bracketed paste; one that
// did not would read the newlines as submissions.
//
// It does not submit. See submitDelay.
func paste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~"
}

// submit is the Enter that sends what paste put at the prompt.
const submit = "\r"

// defaultSubmitDelay is how long a target's PTY has to stay quiet before the
// relay presses the Enter that sends what it pasted (see awaitSettled).
//
// Everything else lich pastes into a prompt is left for the user to send, so
// this is the only place that presses Enter itself — and a carriage return
// arriving while the TUI is still taking the paste in is swallowed. Claude Code
// collapses a multi-line paste into a "[Pasted text #2 +7 lines]" placeholder
// and the Enter goes into building the placeholder rather than sending it;
// Codex, on a terminal that gave it no paste event, spends 120ms deciding
// whether an Enter belongs inside the burst. Both were the same bug on screen:
// the message sitting unsent at the target's prompt, seen only when someone
// opened that session by hand.
//
// It has to outlast the longest of those windows, and it is what a settled
// terminal is measured against, so it is the whole instrument in both roles.
const defaultSubmitDelay = 150 * time.Millisecond

// sanitize strips the control characters that would either break out of the
// bracketed paste framing or drive the target's terminal, keeping the
// whitespace a prompt legitimately contains. ESC is the one that matters: text
// carrying "\x1b[201~" would end the paste early and leave the rest of itself
// running as keystrokes.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
}
