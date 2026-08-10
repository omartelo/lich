// What lich writes at the sender's own prompt when the user picks a session to
// hand work to. lich never carries the request itself: it puts the words there
// and hands the cursor back, and the agent reading that prompt is what acts.

import type { SessionKind } from "./sessions"

// The providers lich registers its MCP server with at spawn. Mirrors
// providers.AcceptsMCPServer on the Go side — the two have to agree, because an
// agent offered a tool it does not have would answer the user with an error.
//
// The session's declared kind, never the live agent readout: a shell card
// running `claude` by hand was spawned without the registration, so it has the
// command and not the tools.
const TOOL_KINDS: readonly string[] = ["claude", "codex"]

/**
 * The text to type at a sender of this kind, addressing the session labelled
 * target.
 *
 * An agent that has lich's tools is given the request in its own language and
 * picks the tool itself; one that has none is given the command, because
 * nothing else would tell it the command exists. The quote is left open on
 * purpose there — the user types the task inside it, which is where the cursor
 * lands.
 */
export function delegatePrompt(senderKind: SessionKind | string, target: string): string {
  if (TOOL_KINDS.includes(senderKind)) {
    return `Ask the "${target}" session to `
  }
  return `lich send "${target}" "`
}
