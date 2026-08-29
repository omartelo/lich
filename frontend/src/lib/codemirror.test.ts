import { describe, expect, it } from "vitest"
import { buildGapDecorations } from "./codemirror"
import { buildFileDoc, parseDiff, type DiffGap } from "./git/diff"

// The widget's own DOM is a framework boundary and is left to the app; what is
// tested here is where its decorations land, and that adding two of them to one
// position is an order a RangeSetBuilder accepts — it throws otherwise, and a
// throw building the diff takes the window down with it.

const gapped = `diff --git a/src/app.ts b/src/app.ts
--- a/src/app.ts
+++ b/src/app.ts
@@ -6,2 +6,2 @@ function head() {
-  const b = 2
+  const b = 3
@@ -20,1 +20,2 @@ function tail() {
   const z = 9
+  const w = 10`

function offsets(gaps: DiffGap[], lineMeta: ReturnType<typeof buildFileDoc>["lineMeta"]): number[] {
  const set = buildGapDecorations(lineMeta, gaps, () => {})
  const found: number[] = []
  set.between(0, 1e6, (from) => {
    found.push(from)
  })
  return found
}

describe("buildGapDecorations", () => {
  it("addresses each open gap at its separator's character offset", () => {
    const doc = buildFileDoc(parseDiff(gapped)[0])
    // Head gap on doc line 1 (offset 0); the second separator follows the empty
    // line and the hunk's two lines.
    expect(doc.gaps.map((gap) => gap.docLine)).toEqual([1, 4])
    const second = 1 + "  const b = 2".length + 1 + "  const b = 3".length + 1
    // Two decorations per gap — the line class and the widget — on one offset.
    expect(offsets(doc.gaps, doc.lineMeta)).toEqual([0, 0, second, second])
  })

  it("draws nothing when no gap is open", () => {
    const doc = buildFileDoc(parseDiff(gapped)[0])
    expect(offsets([], doc.lineMeta)).toEqual([])
  })
})
