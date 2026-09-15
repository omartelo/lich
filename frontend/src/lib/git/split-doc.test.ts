import { describe, expect, it } from "vitest"
import { buildFileDoc, parseDiff } from "./diff"
import { blockOfLine, changeBlocks, revertLinesIn, revertStat } from "./diff-blocks"
import { buildSplitDoc } from "./split-doc"

const diff = `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -1,6 +1,7 @@
 package app
-var a = 1
-var b = 2
+var a = 10
 var c = 3
+var d = 4
+var e = 5
 var f = 6
@@ -20,2 +21,2 @@
 func tail() {
-	old()
+	renamed()`

function unified() {
  const [file] = parseDiff(diff)
  return buildFileDoc(file)
}

describe("changeBlocks", () => {
  it("finds each run of changes with no unchanged line inside", () => {
    // doc: 1 package, 2-4 block, 5 var c, 6-7 block, 8 var f, 9 separator, 10 tail, 11-12 block
    expect(changeBlocks(unified().lineMeta)).toEqual([
      { from: 2, to: 4 },
      { from: 6, to: 7 },
      { from: 11, to: 12 },
    ])
  })

  it("marks every line of a block with the block's first line", () => {
    expect(blockOfLine(unified().lineMeta)).toEqual([
      null,
      2,
      2,
      2,
      null,
      6,
      6,
      null,
      null,
      null,
      11,
      11,
    ])
  })
})

describe("revertLinesIn", () => {
  it("names deletions by old number and additions by new number, with their text", () => {
    const lines = revertLinesIn(unified().lineMeta, 1, 4)
    expect(lines).toEqual([
      { side: "old", line: 2, text: "var a = 1" },
      { side: "old", line: 3, text: "var b = 2" },
      { side: "new", line: 2, text: "var a = 10" },
    ])
    expect(revertStat(lines)).toEqual({ added: 1, deleted: 2 })
  })

  it("is empty for a span holding no change", () => {
    expect(revertLinesIn(unified().lineMeta, 8, 10)).toEqual([])
  })
})

describe("buildSplitDoc", () => {
  it("keeps both sides the same number of rows", () => {
    const split = buildSplitDoc(unified())
    expect(split.left.lineMeta).toHaveLength(split.right.lineMeta.length)
    expect(split.left.text.split("\n")).toHaveLength(split.left.lineMeta.length)
    expect(split.blocks).toHaveLength(split.left.lineMeta.length)
  })

  it("pairs deletions with additions by position and pads the shorter side", () => {
    const { left, right, blocks } = buildSplitDoc(unified())
    const kinds = left.lineMeta.map((line, row) => `${line.kind}|${right.lineMeta[row].kind}`)
    expect(kinds).toEqual([
      "context|context",
      "del|add",
      "del|filler",
      "context|context",
      "filler|add",
      "filler|add",
      "context|context",
      "meta|meta",
      "context|context",
      "del|add",
    ])
    expect(blocks).toEqual([null, 2, 2, null, 6, 6, null, null, null, 11])
    expect(right.text.split("\n")[1]).toBe("var a = 10")
    expect(left.text.split("\n")[2]).toBe("var b = 2")
  })

  it("anchors a row on the old number from the left and the new one from the right", () => {
    const { anchors } = buildSplitDoc(unified())
    expect(anchors[1]).toMatchObject({ kind: "add", oldLine: 2, newLine: 2 })
    expect(anchors[2]).toMatchObject({ kind: "del", oldLine: 3, newLine: null })
    expect(anchors[4]).toMatchObject({ kind: "add", oldLine: null, newLine: 4 })
  })

  it("moves each gap to the row its separator landed on", () => {
    const { right, left } = buildSplitDoc(unified())
    expect(right.gaps).toEqual([{ key: 8, docLine: 8, from: 8, to: 20, oldFrom: 7 }])
    expect(left.gaps).toEqual(right.gaps)
  })
})
