import { describe, expect, it } from "vitest"
import { baseName, displayPath } from "./paths"

describe("displayPath", () => {
  it("collapses a POSIX home prefix to ~", () => {
    expect(displayPath("/home/meopedevts/try/skipo")).toBe("~/try/skipo")
  })

  it("collapses a macOS home prefix to ~", () => {
    expect(displayPath("/Users/me/try/skipo")).toBe("~/try/skipo")
  })

  it("collapses a Windows user profile prefix to ~", () => {
    expect(displayPath("C:\\Users\\me\\try\\skipo")).toBe("~\\try\\skipo")
  })

  it("leaves paths outside a home directory unchanged", () => {
    expect(displayPath("/opt/app/data")).toBe("/opt/app/data")
    expect(displayPath("/home")).toBe("/home")
    expect(displayPath("")).toBe("")
  })
})

describe("baseName", () => {
  it("returns the final POSIX segment", () => {
    expect(baseName("/home/me/try/skipo")).toBe("skipo")
  })

  it("ignores a trailing slash", () => {
    expect(baseName("/home/me/try/skipo/")).toBe("skipo")
  })

  it("handles Windows separators", () => {
    expect(baseName("C:\\Users\\me\\worktrees\\windows-port")).toBe("windows-port")
  })

  it("returns empty for a root or empty path", () => {
    expect(baseName("/")).toBe("")
    expect(baseName("")).toBe("")
  })
})
