// What lich writes at the sender's own prompt when the user picks a session to
// hand work to. lich never carries the request itself: it puts the words there
// and hands the cursor back, and the agent reading that prompt is what acts.

import type { SessionKind } from "./sessions"

// The providers whose sessions are handed lich's tools on their own command
// line at spawn (providers.AcceptsMCPServer). The kind alone answers it, and
// that answer cannot go stale.
//
// opencode and Crush sessions can have the same tools from the companion plugin
// (agentplugin.HasTools), and are still given the command here on purpose. What
// the plugin reports is a fact about what is installed now, not about a session
// started before the install — it only reaches one on its next restart — and an
// agent offered a tool it does not have answers the user with an error. The
// command works in all four and costs nothing.
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
 *
 * The prose form ends in a colon, not a preposition: what follows can be an
 * order, a question, or pasted context, and none of them should have to bend
 * into "to <verb>". The verb is the UI's own — the button says Delegate.
 */
export function delegatePrompt(senderKind: SessionKind | string, target: string): string {
  if (TOOL_KINDS.includes(senderKind)) {
    return `Delegate to the "${target}" session: `
  }
  return `lich send "${target}" "`
}

/**
 * The text for delegating to a session that does not exist yet: the agent
 * opens a worktree session and hands it the task, picking the branch name
 * itself — it knows the task, and the user typed only the task.
 *
 * Prose for every kind, because the branch name is the agent's to pick and it
 * sits before the task in the command — so this cannot be one fill-in command
 * the way `lich send` can, even now that one command carries both. A sender
 * without lich's tools is told the command by name — nothing else would tell it
 * it exists.
 */
export function delegateWorktreePrompt(senderKind: SessionKind | string): string {
  if (TOOL_KINDS.includes(senderKind)) {
    return "Delegate to a new worktree session: "
  }
  return "Delegate to a new worktree session (run `lich open --worktree <branch> --prompt <task>`): "
}
