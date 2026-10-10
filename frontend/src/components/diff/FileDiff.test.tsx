// @vitest-environment jsdom
//
// The harness has to be imported before anything that reaches react-dom (see
// @/test/render-budget), which is why it is first.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import { parseDiff } from "@/lib/git/diff"
import { FileDiff } from "./FileDiff"

vi.mock("@/providers/settings", () => ({
  useSettings: () => ({ hideWhitespace: true, diffLayout: "unified" }),
}))

// A whitespace-only change: with whitespace hidden, an open file draws a one-line
// body and no editor, which jsdom cannot build.
const WHITESPACE_DIFF = `diff --git a/a.txt b/a.txt
index 1111111..2222222 100644
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
-hello
+hello
`

const BODY = "Only whitespace changed"

function fileDiff(viewed: boolean) {
  return createElement(FileDiff, {
    file: parseDiff(WHITESPACE_DIFF)[0],
    onInject: () => {},
    viewed,
    onViewed: () => {},
  })
}

test("an unviewed file opens", async () => {
  const budget = await mountBudget(fileDiff(false))
  expect(document.body.textContent).toContain(BODY)
  await budget.unmount()
})

test("a file already ticked as viewed opens folded", async () => {
  const budget = await mountBudget(fileDiff(true))
  expect(document.body.textContent).not.toContain(BODY)
  await budget.unmount()
})
