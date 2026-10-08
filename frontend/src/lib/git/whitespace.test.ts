import { describe, expect, it } from "vitest"
import { buildFileDoc, parseDiff } from "./diff"
import { revertLinesIn } from "./diff-blocks"
import { hideWhitespace, whitespaceOnly } from "./whitespace"

const header = `diff --git a/src/app.ts b/src/app.ts
index 1111111..2222222 100644
--- a/src/app.ts
+++ b/src/app.ts
`

const reindented = `${header}@@ -1,4 +1,4 @@
 if (ready) {
-run()
-stop()
+  run()
+  stop()
 }`

const mixed = `${header}@@ -1,3 +1,3 @@
 if (ready) {
-run()
+  run(fast)
 }`

const reindentThenRealChange = `${header}@@ -1,3 +1,3 @@
-a  =  1
+a = 1
 keep
@@ -10,3 +10,3 @@ function tail() {
 before
-return b
+return c
 after`

function hidden(text: string) {
  const [original] = parseDiff(text)
  const shown = hideWhitespace(original)
  return { original, shown }
}

describe("hideWhitespace", () => {
  it("turns a re-indented block into unchanged lines showing the file as it is now", () => {
    const { original, shown } = hidden(reindented)
    expect(whitespaceOnly(shown, original)).toBe(true)
    expect(shown.hunks).toEqual([])
    expect(shown.added).toBe(0)
    expect(shown.deleted).toBe(0)
  })

  it("keeps a block that changed more than whitespace whole, prefix and numbers intact", () => {
    const { original, shown } = hidden(mixed)
    expect(shown.hunks).toEqual(original.hunks)
    expect(whitespaceOnly(shown, original)).toBe(false)
  })

  it("drops only the whitespace hunk and recounts from what is left", () => {
    const { shown } = hidden(reindentThenRealChange)
    expect(shown.hunks.map((hunk) => hunk.header)).toEqual(["@@ -10,3 +10,3 @@ function tail() {"])
    expect(shown.added).toBe(1)
    expect(shown.deleted).toBe(1)
  })

  it("numbers a folded line on both sides, so a gap and a comment still find it", () => {
    const text = `${header}@@ -1,3 +1,4 @@
-a  =  1
+a = 1
 keep
+added`
    const { shown } = hidden(text)
    expect(shown.hunks[0].lines[0]).toEqual({
      kind: "context",
      text: " a = 1",
      oldLine: 1,
      newLine: 1,
    })
  })

  it("leaves a block whose sides differ in length alone, even when only blank lines moved", () => {
    const text = `${header}@@ -1,2 +1,3 @@
 a
+
 b`
    const { original, shown } = hidden(text)
    expect(shown.hunks).toEqual(original.hunks)
  })

  it("never offers a hidden line to revert", () => {
    const { shown } = hidden(reindentThenRealChange)
    const doc = buildFileDoc(shown)
    expect(revertLinesIn(doc.lineMeta, 1, doc.lineMeta.length)).toEqual([
      { side: "old", line: 11, text: "return b" },
      { side: "new", line: 11, text: "return c" },
    ])
  })

  it("passes a binary file through untouched", () => {
    const [binary] = parseDiff(`diff --git a/logo.png b/logo.png
index 1111111..2222222 100644
Binary files a/logo.png and b/logo.png differ`)
    expect(hideWhitespace(binary)).toBe(binary)
  })
})
