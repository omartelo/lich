import { describe, expect, it } from "vitest"
import type { BinaryCheck } from "./api-types"
import { checkDetail, checkLabel, failed, winningScope } from "./binary-layers"

const check = (status: BinaryCheck["status"], path = ""): BinaryCheck => ({ path, status })

describe("winningScope", () => {
  it("prefers the project override, then the global one", () => {
    expect(winningScope("/opt/next/claude", "/opt/claude")).toBe("project")
    expect(winningScope("", "/opt/claude")).toBe("global")
    expect(winningScope("", "")).toBe("path")
  })

  // A field holding spaces is a field nobody filled in — and the Go side trims
  // before storing, so a value that reached the store this way is whitespace
  // from an older build.
  it("treats a whitespace-only value as unset", () => {
    expect(winningScope("   ", "")).toBe("path")
    expect(winningScope(" \t ", "/opt/claude")).toBe("global")
  })
})

describe("checkLabel", () => {
  it("says how a missing binary was looked for", () => {
    expect(checkLabel(check("not-found"), "claude")).toBe("not on $PATH")
    expect(checkLabel(check("not-found"), "/opt/gone/claude")).toBe("no such file")
    expect(checkLabel(check("not-found"), "C:\\tools\\claude.exe")).toBe("no such file")
  })

  it("names the two mistakes a shell would have hidden", () => {
    expect(checkLabel(check("home-shortcut"), "~/dev/claude")).toBe("~ is not expanded")
    expect(checkLabel(check("relative"), "bin/claude")).toBe("relative path")
  })

  it("says nothing for a check that has not arrived or has nothing to report", () => {
    expect(checkLabel(null, "claude")).toBe("")
    expect(checkLabel(check("empty"), "")).toBe("")
  })

  it("confirms a binary that resolves", () => {
    expect(checkLabel(check("ok", "/usr/bin/claude"), "claude")).toBe("executable")
  })
})

describe("checkDetail", () => {
  it("explains only the states that need fixing", () => {
    expect(checkDetail(check("home-shortcut"))).toContain("taken literally")
    expect(checkDetail(check("relative"))).toContain("per session")
    expect(checkDetail(check("not-found"))).toContain("will not start")
    expect(checkDetail(check("not-executable"))).toContain("will not start")
    expect(checkDetail(check("ok"))).toBe("")
    expect(checkDetail(null)).toBe("")
  })
})

describe("failed", () => {
  // A check in flight is not a failure: drawn as one, a field being typed into
  // would flash red between keystrokes.
  it("is false while a check is in flight, and for an unset layer", () => {
    expect(failed(null)).toBe(false)
    expect(failed(check("empty"))).toBe(false)
  })

  it("is true for every state a session cannot spawn from", () => {
    for (const status of ["not-found", "not-executable", "home-shortcut", "relative"] as const) {
      expect(failed(check(status))).toBe(true)
    }
    expect(failed(check("ok"))).toBe(false)
  })
})
