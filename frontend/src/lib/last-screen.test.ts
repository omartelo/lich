import { describe, expect, it } from "vitest"
import { resumableScreen } from "./last-screen"

describe("resumableScreen", () => {
  it("lands on a screen of a project that is still open", () => {
    expect(resumableScreen("/projects/p1", ["p1", "p2"])).toBe("/projects/p1")
    expect(resumableScreen("/projects/p2/pulls/all/42", ["p1", "p2"])).toBe(
      "/projects/p2/pulls/all/42",
    )
  })

  it("falls back to Home for a closed project, Home itself or nothing saved", () => {
    expect(resumableScreen("/projects/gone", ["p1"])).toBeNull()
    expect(resumableScreen("/", ["p1"])).toBeNull()
    expect(resumableScreen("", ["p1"])).toBeNull()
    expect(resumableScreen("/projects/", ["p1"])).toBeNull()
  })
})
