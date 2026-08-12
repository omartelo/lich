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
		"sent any other way is lost."
}

// unreadNotice is what a sender who stopped waiting is told, typed at its own
// prompt the way an answer would be. It has to be actionable on its own: the
// sender cannot see the other screen, and what is usually on it is a question
// only a person can answer.
func unreadNotice(target string) string {
	return fmt.Sprintf(
		"[lich] The %q session never picked up the task you sent it. It was typed at "+
			"that prompt and nothing read it, so something else has that terminal — a "+
			"provider still starting, or a question of its own on screen. Nothing is "+
			"queued and nothing was answered.",
		target,
	)
}

// stalledNotice is what a sender who stopped waiting is told when the target
// ended its turn without replying through lich, typed at its own prompt the way
// an answer would be. The pending result promised that prompt an answer, so the
// promise has to be withdrawn the same way it would have been kept — whatever
// the target produced is on its own screen, which only a person can go read.
func stalledNotice(target string) string {
	return fmt.Sprintf(
		"[lich] The %q session finished its turn without answering through lich. "+
			"Nothing more is coming back on that ticket — whatever it produced is on "+
			"that session's own screen, so ask the user to open the %q card to read it.",
		target, target,
	)
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
