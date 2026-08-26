import type { LastTurn } from "@/lib/api-types"

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
