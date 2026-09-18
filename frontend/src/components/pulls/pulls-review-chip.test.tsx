// @vitest-environment jsdom
//
// The status line's review chip. GitHub keeps a CHANGES_REQUESTED standing until
// the reviewer moves, so the verdict on its own cannot say whether there is
// anything left to move it for. The threads behind it can, and this is what
// the chip does with them.
//
// The harness is imported before anything that reaches react-dom, for the reason
// render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, it } from "vitest"
import { ReviewStat } from "./PullsStats"

const NONE = { open: 0, total: 0 }

/** What the chip reads, with the whitespace the icon leaves behind collapsed. */
async function chipText(decision: string, threads = NONE): Promise<string> {
  const budget = await mountBudget(createElement(ReviewStat, { decision, threads }))
  const text = document.body.textContent?.replace(/\s+/g, " ").trim() ?? ""
  await budget.unmount()
  document.body.innerHTML = ""
  return text
}

it("says nothing where the repository asks for no review", async () => {
  expect(await chipText("")).toBe("")
})

it("leaves every other verdict as it was", async () => {
  // Threads are on screen for these too; they are not an answer to the verdict.
  expect(await chipText("APPROVED", { open: 2, total: 3 })).toBe("Approved")
  expect(await chipText("REVIEW_REQUIRED", { open: 2, total: 3 })).toBe("Review required")
})

it("stays bare when the review opened no thread at all", async () => {
  expect(await chipText("CHANGES_REQUESTED")).toBe("Changes requested")
})

it("counts what is still open against every thread there is", async () => {
  expect(await chipText("CHANGES_REQUESTED", { open: 2, total: 3 })).toBe(
    "Changes requested · 2 of 3 threads unresolved",
  )
})

// The complaint this exists for: the branch is done and the chip was still
// reading like the day the review landed.
it("says the branch is done once nothing is open", async () => {
  expect(await chipText("CHANGES_REQUESTED", { open: 0, total: 3 })).toBe(
    "Changes requested · all threads resolved",
  )
})
