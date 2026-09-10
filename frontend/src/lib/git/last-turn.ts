import type { LastTurn } from "@/lib/api-types"
import type { SessionStatus } from "@/lib/session/session-events"
import type { SessionKind } from "@/lib/session/sessions"

// What the Review panel draws for the session's last finished turn. The three
// answers are kept apart because conflating them is the one mistake this
// feature can make: a turn that ran and changed nothing is a real answer, and
// "nobody recorded a turn here" is the absence of one — reported as the first,
// it reads as "the agent did nothing".
export type LastTurnNotice = "diff" | "empty" | "unrecorded"

// lastTurnNotice weighs the backend's own state against what parseDiff could
// make of the text. An "ok" carrying nothing a file list can be built from is
// unrecorded rather than empty: "empty" is the backend's word for two identical
// trees, and it says so itself.
export function lastTurnNotice(state: LastTurn["state"] | null, fileCount: number): LastTurnNotice {
  if (state === "empty") {
    return "empty"
  }
  if (state === "ok" && fileCount > 0) {
    return "diff"
  }
  return "unrecorded"
}

// turnSwitchable is whether the Review panel offers the "Last turn" source at
// all. Either half is enough. A session that has reported its state has a turn
// boundary, so the switch is earned even before the first turn closes; a
// session holding a record has one on file already, which is the case a
// restored card lands in — it may not report again for hours, and withholding
// the switch until it does would hide a turn the backend is already holding.
//
// A provider that reports nothing and has never recorded a turn is still
// offered the working tree alone, which is the whole point of the gate.
export function turnSwitchable(everReported: boolean, hasLastTurn: boolean): boolean {
  return everReported || hasLastTurn
}

// The two providers whose CLI reports neither the start nor the end of a turn,
// under the name a user reads in Settings, mirrored from
// docs/hooks/session-state.md (keep in sync). Crush registers no state at all,
// and Cursor CLI's reports are dropped by closableState, so on both of them
// nothing ever opens a window for the panel to bracket.
//
// Written out one by one rather than derived, for the same reason
// NO_FORK_PROVIDERS is (sessions.ts): a provider added to lich has to answer
// this question on purpose instead of inheriting a claim nobody measured.
const NO_TURN_PROVIDERS: Partial<Record<SessionKind, string>> = {
  crush: "Crush",
  cursor: "Cursor CLI",
}

// turnUnavailableReason is the sentence the Review panel wears where the "Last
// turn" source is dead, and "" wherever the absence is not the provider's: a
// shell, and a session whose provider does report but has yet to say anything.
// That second one is a switch about to appear, and naming a provider there
// would be a lie half a second long.
//
// The panel draws the dead switch under this rather than dropping the strip,
// exactly as the card's Fork item does (SessionForkItem): a user who reviews a
// Claude Code turn and finds no such control on their Crush card has no way to
// learn the offer was withheld rather than missing.
export function turnUnavailableReason(kind: SessionKind | ""): string {
  const name = kind === "" ? undefined : NO_TURN_PROVIDERS[kind]
  return name === undefined
    ? ""
    : `${name} reports neither the start nor the end of a turn, so there is no window to bracket.`
}

// saidNote is what the recap band says about whose words it is showing. The
// band reads the last thing the agent said, which is not always the turn the
// diff beside it brackets: while a turn is running those are still the previous
// turn's words, next to a diff that reads "unavailable" until the window closes.
//
// "" for every other state, and deliberately including "waiting": that report
// covers both a session blocked mid-turn and one sitting at its prompt with the
// turn already over, and a label naming the wrong turn is worse than none.
export function saidNote(status: SessionStatus | null): string {
  return status === "busy" ? "from the previous turn" : ""
}
