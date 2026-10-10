import type { ProviderKind, SessionKind } from "./sessions"
import { t } from "@/lib/i18n/i18n"
import { formatUnit } from "@/lib/i18n/unit-format"

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
  const parts = handsOnParts(seconds)
  if (!parts) {
    return ""
  }
  // Units that carry no space of their own ("1h", "12m", "1小时") sit flush, as
  // the strip always did; "1 h" and "12 min" keep a space between them.
  return parts.some((part) => /\s/.test(part)) ? parts.join(" ") : parts.join("")
}

// spellHandsOn is the same figure with room to breathe, for the tooltip: "1h
// 12m", "48m". Same rules, one space — the strip is scanned, the tooltip is
// read.
export function spellHandsOn(seconds: number): string {
  return handsOnParts(seconds)?.join(" ") ?? ""
}

// handsOnParts is the hours and minutes of the figure, each with its unit in the
// interface language. Minutes pad to two digits behind an hour, but only where
// the unit is a Latin abbreviation: "1h05m" reads, "1小时05分钟" does not.
function handsOnParts(seconds: number): string[] | null {
  if (!Number.isFinite(seconds) || seconds < 60) {
    return null
  }
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  if (hours === 0) {
    return [formatUnit(minutes, "minute")]
  }
  const rest = formatUnit(minutes % 60, "minute")
  const padded = /^\d+\s?\p{Script=Latin}/u.test(rest)
    ? rest.replace(/^\d+/, (n) => n.padStart(2, "0"))
    : rest
  return [formatUnit(hours, "hour"), padded]
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
  cursor: "turn",
  // Kiro opens a turn: lich registers userPromptSubmit for busy and stop for
  // done, so a turn the user never interrupts is still counted from end to end.
  kiro: "turn",
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
  // Keep the 15 minutes in sync with handsOnIdleGap in internal/terminal/handson.go.
  return kind && kind !== "shell" && RUNG[kind] === "tool"
    ? t("session.handsOn.detailTool")
    : t("session.handsOn.detailTurn")
}
