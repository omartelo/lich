import type { ProviderKind, SessionKind } from "./sessions"

// formatHandsOn renders how long a session has been worked on, for the footer,
// where it sits beside the cost figure at 12px and is glanced at rather than
// read: "48m", "1h12m", "14h05m".
//
// Minutes are the floor. A session under one is a session that has counted
// nothing worth a figure — the store is up to a flush behind besides — and "0m"
// reads as a broken readout rather than as a new session, so it renders as
// nothing at all and the segment does not appear.
//
// The minutes are zero-padded once there are hours in front of them, because
// "1h5m" and "1h50m" differ by one glyph in a strip nobody stops to parse.
export function formatHandsOn(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 60) {
    return ""
  }
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  if (hours === 0) {
    return `${minutes}m`
  }
  return `${hours}h${String(minutes % 60).padStart(2, "0")}m`
}

// spellHandsOn is the same figure with room to breathe, for the tooltip: "1h
// 12m", "48m". Same rules, one space — the strip is scanned, the tooltip is
// read.
export function spellHandsOn(seconds: number): string {
  return formatHandsOn(seconds).replace("h", "h ")
}

// Which signals a provider's session can beat the hands-on clock with —
// internal/providers.Registry is the checklist, docs/ceilings.md holds the why.
// A "turn" provider's hooks open a turn, so a turn nobody touches is counted
// from the session's own output; a "tool" provider never opens one, and the
// clock hears that session only through the reports its tool calls fire. Every
// provider is spelled out rather than defaulted, so a new one has to pick a
// side here instead of inheriting a sentence that may not be true of it.
const RUNG: Record<ProviderKind, "turn" | "tool"> = {
  claude: "turn",
  codex: "turn",
  antigravity: "turn",
  opencode: "turn",
  omp: "turn",
  crush: "tool",
  cursor: "tool",
}

// handsOnDetail is the sentence under the figure in the tooltip: what the clock
// listened to, and what it let go. Only the middle clause moves — naming a turn
// on a provider that never opens one describes something that does not happen
// there — and both rungs say it in the same two sentences, because a tooltip
// that runs longer on one provider than another is lich explaining itself
// rather than reading its own figure back.
//
// A plain shell keeps the turn wording: it reports nothing either way, and the
// clause names what the clock listens for rather than promising the session
// does all three.
export function handsOnDetail(kind: SessionKind | ""): string {
  const beats =
    kind && kind !== "shell" && RUNG[kind] === "tool"
      ? "typed at, or reporting a tool call"
      : "typed at, reporting, or running a turn"
  // Keep the 15 minutes in sync with handsOnIdleGap in internal/terminal/handson.go.
  return `How long this session has been worked on — ${beats}. A gap longer than 15 minutes counts as time away.`
}
