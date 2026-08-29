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
