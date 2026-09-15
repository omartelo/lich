import { Text } from "@codemirror/state"
import { describe, expect, it } from "vitest"
import { buildRevertDecorations, firstLines } from "./codemirror-revert"

// The button's own DOM is a framework boundary; what is pinned here is where the
// tint and the button land, and that both on one line is an order the builder
// accepts; it throws otherwise, and a throw building a diff takes the window.

const doc = Text.of(["a", "b", "c", "d", "e"])
const blocks = [null, 2, 2, null, 5]
const noop = () => {}

function found(hot: number | null, buttons?: ReadonlyMap<number, number>) {
  const set = buildRevertDecorations(doc, { blocks, buttons, onRevert: noop }, hot)
  const out: string[] = []
  set.between(0, doc.length, (from, _to, value) => {
    out.push(`${from}:${value.spec.widget ? "button" : "tint"}`)
  })
  return out
}

describe("buildRevertDecorations", () => {
  it("draws nothing while no block is hot", () => {
    expect(found(null, firstLines(blocks))).toEqual([])
  })

  it("tints every line of the hot block and puts the button at its first line's end", () => {
    // "b" starts at offset 2 and ends at 3; "c" starts at 4.
    expect(found(2, firstLines(blocks))).toEqual(["2:tint", "3:button", "4:tint"])
  })

  it("only tints in a view that draws no button", () => {
    expect(found(2)).toEqual(["2:tint", "4:tint"])
  })

  it("ignores a block row past the end of the document", () => {
    const short = Text.of(["a"])
    const set = buildRevertDecorations(short, { blocks: [null, 2], onRevert: noop }, 2)
    expect(set.size).toBe(0)
  })
})

describe("firstLines", () => {
  it("maps each block to the first line it covers", () => {
    expect([...firstLines(blocks)]).toEqual([
      [2, 2],
      [5, 5],
    ])
  })
})
