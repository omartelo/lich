import { describe, expect, it } from "vitest"
import { checkoutLabel } from "./checkout-label"

const PROJECT = "/home/me/code/lich"
const ID = "e48f04f46a4a"
const ROOT = `/home/me/.local/share/lich/worktrees/${ID}`

describe("checkoutLabel", () => {
  it("titles the root group with the project folder", () => {
    expect(checkoutLabel("", PROJECT, ID)).toBe("lich")
  })

  it("keeps the whole worktree name when it holds a slash", () => {
    expect(checkoutLabel(`${ROOT}/feat/x`, PROJECT, ID)).toBe("feat/x")
  })

  it("tells apart two worktrees sharing a last segment", () => {
    expect(checkoutLabel(`${ROOT}/feat/x`, PROJECT, ID)).not.toBe(
      checkoutLabel(`${ROOT}/fix/x`, PROJECT, ID),
    )
  })

  it("names a slashless worktree the same as before", () => {
    expect(checkoutLabel(`${ROOT}/eager-willow`, PROJECT, ID)).toBe("eager-willow")
  })

  it("reads a Windows checkout, which lich spells with backslashes", () => {
    expect(
      checkoutLabel(`C:\\Users\\me\\AppData\\lich\\worktrees\\${ID}\\feat\\x`, PROJECT, ID),
    ).toBe("feat/x")
  })

  it("cuts at the root, not at a worktree named after the project id", () => {
    expect(checkoutLabel(`${ROOT}/${ID}/x`, PROJECT, ID)).toBe(`${ID}/x`)
  })

  it("falls back to the last segment for a checkout outside the worktree root", () => {
    expect(checkoutLabel("/home/me/code/lich-elsewhere", PROJECT, ID)).toBe("lich-elsewhere")
  })
})
