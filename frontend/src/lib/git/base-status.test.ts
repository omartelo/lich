import { describe, expect, it } from "vitest"
import type { BaseStatus } from "@/lib/api-types"
import { baseReadout } from "./base-status"

const status = (over: Partial<BaseStatus> = {}): BaseStatus => ({
  base: "origin/main",
  behind: 0,
  conflicts: null,
  ...over,
})

describe("baseReadout", () => {
  it("draws nothing when the service has no answer", () => {
    expect(baseReadout(null)).toBeNull()
  })

  it("draws nothing when the base has not moved", () => {
    expect(baseReadout(status())).toBeNull()
  })

  it("counts commits when the base moved and the merge is clean", () => {
    expect(baseReadout(status({ behind: 3 }))).toEqual({
      kind: "behind",
      count: 3,
      behind: "3 commits behind origin/main",
      conflict: null,
      paths: [],
      more: 0,
    })
  })

  it("counts files, not commits, once the merge collides", () => {
    const got = baseReadout(status({ behind: 3, conflicts: ["a.go", "b.go"] }))
    expect(got?.kind).toBe("conflict")
    expect(got?.count).toBe(2)
    expect(got?.behind).toBe("3 commits behind origin/main")
    expect(got?.conflict).toBe("Conflicts in 2 files")
  })

  it("singularises one commit and one file", () => {
    const got = baseReadout(status({ behind: 1, conflicts: ["a.go"] }))
    expect(got?.behind).toBe("1 commit behind origin/main")
    expect(got?.conflict).toBe("Conflicts in 1 file")
  })

  it("names the base branch it was measured against", () => {
    expect(baseReadout(status({ behind: 2, base: "origin/develop" }))?.behind).toBe(
      "2 commits behind origin/develop",
    )
  })

  // The cap is pinned as a literal on both sides: derived from the constant, a
  // test would follow the cap wherever it moved and stop guarding the tooltip's
  // height, which is the only reason the cap exists.
  it("names every path up to three", () => {
    const got = baseReadout(status({ behind: 1, conflicts: ["a", "b", "c"] }))
    expect(got?.paths).toEqual(["a", "b", "c"])
    expect(got?.more).toBe(0)
  })

  it("counts the rest past three", () => {
    const got = baseReadout(status({ behind: 1, conflicts: ["a", "b", "c", "d", "e"] }))
    expect(got?.paths).toEqual(["a", "b", "c"])
    expect(got?.more).toBe(2)
  })

  // A conflict with the base commit already merged is not a state the service
  // produces, but the readout must not vanish if it ever does: the alert is the
  // half that matters, and dropping it would hide a real collision.
  it("still alerts when conflicts arrive without a behind count", () => {
    const got = baseReadout(status({ behind: 0, conflicts: ["a.go"] }))
    expect(got?.kind).toBe("conflict")
    expect(got?.behind).toBeNull()
  })
})
