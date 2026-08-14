package relay

import (
	"fmt"
	"strings"
	"time"
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
func SpawnBriefing(hasTools bool) string {
	route := "Open one with `lich open --worktree BRANCH --prompt 'the task'`, which opens the " +
		"session and hands it the task in one command."
	if hasTools {
		route = "The lich tools in your list open one and hand it the task."
	}
	return "You are running inside lich, which runs coding-agent sessions side by side and can " +
		"open more of them beside this one — each a card the user watches and can take over " +
		"mid-task, in its own git worktree when the work needs its own checkout. When work is " +
		"to be fanned out — several tasks at once, one per branch or checkout — those sessions " +
		"are what to open, not the subagents your own harness runs: a subagent has no checkout, " +
		"no card, and nothing the user can steer or resume. " + route
}

// compose is the message typed at the target's prompt. It names where the
// request came from so the receiving agent knows this did not come from the
// person in front of it, and carries the exact command that sends an answer
// home — the reply path only exists because this text describes it, so the
// agent needs no prior knowledge of the feature.
func compose(sender, ticketID, prompt string, hasTools bool) string {
	return fmt.Sprintf(
		"[lich] %s, not from your own prompt.\n\n%s\n\n%s",
		origin(sender), prompt, replyInstruction(hasTools, ticketID),
	)
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
func replyInstruction(hasTools bool, ticketID string) string {
	command := fmt.Sprintf("  \"$LICH_BIN\" reply %s \"<your answer>\"", ticketID)
	route := "When you have an answer, send it back by running:\n" + command
	if hasTools {
		route = fmt.Sprintf(
			"When you have an answer, send it back with the lich tool `%s` (ticket %s), or by running:\n%s",
			ToolReply, ticketID, command,
		)
	}
	return route + "\n\nThat ticket is the only way back: whoever asked is blocked on it and " +
		"is reading nothing else. Do not answer by messaging a peer session — an answer " +
		"sent any other way is lost. Keep the answer a concise report — what was done, " +
		"where, and what remains — never a transcript: the sender pays to read every byte, " +
		"and the detail is in your commits and files anyway."
}

// nudgeNotice is the one line typed at a sender's prompt when results are
// waiting and nobody is holding the line for them. It replaces typing the
// results themselves: N results landing as N prompt submissions each restart
// the sender's turn and leave the full text sitting in its context window,
// while one short line lets the agent drain everything in a single tool call.
// count and targets cover everything waiting, not only what this nudge is the
// first to mention — the reader acts on the total.
func nudgeNotice(count int, targets []string, hasTools bool) string {
	what := fmt.Sprintf("Results from %d tasks you sent are ready (%s)", count, quotedList(targets))
	if count == 1 {
		what = fmt.Sprintf("The task you sent %s has its result ready", quotedList(targets))
	}
	route := "run:\n  \"$LICH_BIN\" wait"
	if hasTools {
		route = fmt.Sprintf(
			"call the lich tool `%s` with no ticket, or run:\n  \"$LICH_BIN\" wait", ToolCollect,
		)
	}
	return fmt.Sprintf("[lich] %s. To collect everything at once, %s", what, route)
}

// quotedList words a list of session labels for a message.
func quotedList(labels []string) string {
	quoted := make([]string, 0, len(labels))
	for _, label := range labels {
		quoted = append(quoted, fmt.Sprintf("%q", label))
	}
	return strings.Join(quoted, ", ")
}

// origin describes the sender in the message's first line. An empty sender is
// the lich CLI run outside any session — a script, a scheduled job, the user's
// own shell — which is a different thing to be told than "another agent".
func origin(sender string) string {
	if sender == "" {
		return "Message relayed by the lich command line"
	}
	return fmt.Sprintf("Message from session %q", sender)
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

// defaultSubmitDelay is how long the relay waits between the paste and the
// Enter that sends it.
//
// Everything else lich pastes into a prompt is left for the user to send, so
// this is the only place that presses Enter itself — and a carriage return
// riding in the same write as the paste is swallowed. Claude Code collapses a
// multi-line paste into a "[Pasted text #2 +7 lines]" placeholder, and the
// Enter arriving inside that same burst goes into building the placeholder
// rather than sending it: the message sat unsent at the target's prompt, seen
// only when someone opened that session by hand. Nothing here can read the
// target's screen to know when it has settled, so the delay is the instrument,
// and it is generous on purpose — a tenth of a second nobody notices against a
// message that otherwise never arrives.
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
