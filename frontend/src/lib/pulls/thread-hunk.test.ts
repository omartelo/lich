import { describe, expect, it } from "vitest"
import { threadHunk } from "./thread-hunk"

// A hunk shaped like the one an added file produces: every line new, numbered
// from the header, ending on the line the comment was filed on.
function addedFile(upTo: number): string {
  const lines = ["@@ -0,0 +1,120 @@"]
  for (let n = 1; n <= upTo; n += 1) {
    lines.push(`+line ${n}`)
  }
  return lines.join("\n")
}

describe("threadHunk", () => {
  it("cuts a multi-line thread down to the span it covers", () => {
    const lines = threadHunk(addedFile(74), { line: 74, startLine: 65, side: "RIGHT" })
    expect(lines.map((l) => l.newLine)).toEqual([65, 66, 67, 68, 69, 70, 71, 72, 73, 74])
    expect(lines[0].text).toBe("+line 65")
  })

  it("keeps a little context above a single-line thread", () => {
    const lines = threadHunk(addedFile(74), { line: 74, startLine: 0, side: "RIGHT" })
    expect(lines.map((l) => l.newLine)).toEqual([71, 72, 73, 74])
  })

  it("falls back to the hunk's tail when the anchor moved off it", () => {
    // A re-anchored thread: line 900 is nowhere in the hunk, but the hunk still
    // ends on the line it was filed on.
    const lines = threadHunk(addedFile(74), { line: 900, startLine: 0, side: "RIGHT" })
    expect(lines.map((l) => l.newLine)).toEqual([71, 72, 73, 74])
  })

  it("caps a span nobody wants to scroll", () => {
    const lines = threadHunk(addedFile(74), { line: 74, startLine: 2, side: "RIGHT" })
    expect(lines).toHaveLength(24)
    expect(lines[lines.length - 1].newLine).toBe(74)
  })

  it("numbers a deleted line on the side it was deleted from", () => {
    const hunk = ["@@ -57,4 +57,3 @@", " kept", "-gone", " after", "+added"].join("\n")
    const lines = threadHunk(hunk, { line: 58, startLine: 0, side: "LEFT" })
    expect(lines.map((l) => [l.kind, l.oldLine, l.newLine])).toEqual([
      ["context", 57, 57],
      ["del", 58, null],
      ["context", 59, 58],
      ["add", null, 59],
    ])
  })

  it("survives a hunk it cannot parse", () => {
    expect(threadHunk("", { line: 12, startLine: 0, side: "RIGHT" })).toEqual([])
  })
})
