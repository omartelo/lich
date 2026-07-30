import { Text } from "@codemirror/state"
import { describe, expect, it } from "vitest"
import { buildSlots, type ThreadSlot } from "./codemirror-threads"

// The widget's own DOM is a framework boundary and is left to the app; what is
// tested here is the mapping the widget hangs off — the line a gap is anchored
// to, and which gaps survive being anchored at all.
//
// line 1 "a"   → 0..1
// line 2 "bb"  → 2..4
// line 3 "ccc" → 5..8
const doc = Text.of(["a", "bb", "ccc"])

const built = (slots: ThreadSlot[]) => {
  const set = buildSlots(doc, slots, () => {})
  const out: { key: string; at: number }[] = []
  for (const cursor = set.iter(); cursor.value !== null; cursor.next()) {
    out.push({ key: cursor.value.spec.widget.key, at: cursor.from })
  }
  return out
}

describe("buildSlots", () => {
  it("anchors a gap at the end of its line, so it opens under the code", () => {
    expect(built([{ key: "t:a", docLine: 2 }])).toEqual([{ key: "t:a", at: 4 }])
  })

  it("sorts what it is handed — a RangeSet built out of order throws", () => {
    const slots = [
      { key: "t:c", docLine: 3 },
      { key: "t:a", docLine: 1 },
      { key: "t:b", docLine: 2 },
    ]
    expect(built(slots).map((slot) => slot.at)).toEqual([1, 4, 8])
  })

  it("orders two gaps on one line by key, so the same pair never swaps places", () => {
    const slots = [
      { key: "t:thread", docLine: 2 },
      { key: "d:RIGHT:2-2", docLine: 2 },
    ]
    expect(built(slots).map((slot) => slot.key)).toEqual(["d:RIGHT:2-2", "t:thread"])
  })

  it("drops a gap the document cannot hold instead of clamping it onto a neighbour", () => {
    // Past the last line: a thread on code this diff no longer renders.
    expect(built([{ key: "t:gone", docLine: 4 }])).toEqual([])
    // Before the first: a line number that never resolved.
    expect(built([{ key: "t:none", docLine: 0 }])).toEqual([])
  })

  it("keeps the gaps it can when only some of them anchor", () => {
    const slots = [
      { key: "t:gone", docLine: 99 },
      { key: "t:here", docLine: 1 },
    ]
    expect(built(slots)).toEqual([{ key: "t:here", at: 1 }])
  })
})
