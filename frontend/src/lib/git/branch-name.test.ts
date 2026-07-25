import { describe, expect, it } from "vitest"
import { isValidBranchName } from "./branch-name"

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
