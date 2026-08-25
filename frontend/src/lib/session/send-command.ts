// The `lich send` line that reaches one session, written out so it can be
// pasted into another terminal instead of retyped from memory. The card knows
// the two things the line needs — the label the session answers to and the
// project it belongs to — and neither is anywhere else once the window is out
// of sight.

import type { Project } from "@/lib/api-types"
import { sessionsOf, type Session, type SessionState } from "./sessions"

/**
 * Quote a word for a POSIX shell: the page-side half of internal/shquote, which
 * quotes the command lines the backend composes the same way. One rule in two
 * languages, because a label with a space, a quote or a `$` in it has to paste
 * back as the single argument the user named the card.
 */
export function shQuote(value: string): string {
  return `'${value.split("'").join(`'\\''`)}'`
}

/**
 * The same word for PowerShell, which is the shell a Windows box drops the user
 * into — and, since lich opens its own Windows sessions there, the shell this
 * line is most often pasted back into. Single quotes expand nothing there
 * either; only the escape differs, doubling the quote instead of closing and
 * reopening the run. The page-side half of shquote.QuotePwsh.
 */
export function pwshQuote(value: string): string {
  return `'${value.split("'").join("''")}'`
}

// Left for the user to fill in, spelled as `lich send --help` spells it. Double
// quotes rather than the shQuote rule: what goes here is prose, and an
// apostrophe in it must not end the argument.
const PROMPT = '"<prompt>"'

/**
 * The command that hands this session a task from anywhere else.
 *
 * isWindows picks the quoting rule, the way composeDroppedPaths does: the line
 * is pasted into a terminal on this machine, so the shell that matters is the
 * user's own, not the session's.
 *
 * `--project` is written only when it decides something. `lich send` resolves a
 * label on its own and asks for the project exactly when the same label names
 * more than one live session (internal/relay/roster.go, resolve) — and a label
 * is unique inside a project, so a second holder is always in another one.
 * Unconditionally naming the project would be noise on every card but that one.
 */
export function sendCommand(
  projects: readonly Project[],
  sessions: SessionState,
  session: Session,
  isWindows = false,
): string {
  const quote = isWindows ? pwshQuote : shQuote
  const all = projects.flatMap((project) =>
    sessionsOf(sessions, project.id).map((candidate) => ({ project, candidate })),
  )
  const own = all.find((entry) => entry.candidate.id === session.id)
  const shared = all.some(
    (entry) =>
      entry.candidate.id !== session.id &&
      entry.candidate.label.toLowerCase() === session.label.toLowerCase(),
  )
  const scope = shared && own ? ` --project ${quote(own.project.name)}` : ""
  return `lich send${scope} ${quote(session.label)} ${PROMPT}`
}
