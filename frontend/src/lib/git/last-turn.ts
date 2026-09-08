import type { LastTurn } from "@/lib/api-types"
import type { SessionStatus } from "@/lib/session/session-events"

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
