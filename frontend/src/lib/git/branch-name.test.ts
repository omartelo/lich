import { describe, expect, it } from "vitest"
import { isValidBranchName, toBranchName } from "./branch-name"

describe("isValidBranchName", () => {
  const valid = ["main", "feat/x", "a-b_c.d", "v1.0", "wt/deep/nesting", "UPPER"]
  it.each(valid)("accepts %j", (name) => {
    expect(isValidBranchName(name)).toBe(true)
  })

  const invalid = [
    "",
    "@",
    "has space",
    "tab\there",
    "a..b",
    "a@{b",
    "-x",
    ".x",
    "x.",
    "x.lock",
    "x/",
    "/x",
    "a//b",
    "a/.b",
    "a/b.lock",
    "fe~at",
    "fe^at",
    "fe:at",
    "fe?at",
    "fe*at",
    "fe[at",
    "fe\\at",
  ]
  it.each(invalid)("rejects %j", (name) => {
    expect(isValidBranchName(name)).toBe(false)
  })
})

describe("toBranchName", () => {
  const kept = ["main", "feat/x", "a-b_c.d", "wt/deep/nesting", "!!!"]
  it.each(kept)("keeps %j, which git already accepts", (name) => {
    expect(toBranchName(name)).toBe(name)
  })

  it("trims before deciding", () => {
    expect(toBranchName("  feat/x  ")).toBe("feat/x")
  })

  it("slugifies a typed phrase", () => {
    expect(toBranchName("Fix the auth redirect loop")).toBe("fix-the-auth-redirect-loop")
  })

  it("keeps letters that are not ASCII", () => {
    expect(toBranchName("corrigir a sessão órfã")).toBe("corrigir-a-sessão-órfã")
  })

  it("stops at five words", () => {
    expect(toBranchName("one two three four five six")).toBe("one-two-three-four-five")
  })

  it("stops before the character maximum once the minimum is met", () => {
    expect(toBranchName("refactor the authentication middleware completely")).toBe(
      "refactor-the-authentication-middleware",
    )
  })

  it("grows past the character maximum while under the character minimum", () => {
    expect(toBranchName(`go on ${"z".repeat(40)}`)).toBe(`go-on-${"z".repeat(34)}`)
  })

  it("clamps a word that alone runs past the maximum", () => {
    expect(toBranchName(`${"z".repeat(50)} x`)).toBe("z".repeat(40))
  })

  it("leaves no trailing separator when the clamp lands on one", () => {
    expect(toBranchName(`${"a".repeat(39)} b`)).toBe("a".repeat(39))
  })

  const nothing = ["", "   ", "@", "..."]
  it.each(nothing)("returns nothing for %j, so the caller auto-generates", (input) => {
    expect(toBranchName(input)).toBe("")
  })
})
