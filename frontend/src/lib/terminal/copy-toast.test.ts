import { describe, expect, it } from "vitest"
import { copyToastMessage, countChars } from "./copy-toast"

describe("countChars", () => {
  it("counts ascii by length", () => {
    expect(countChars("hello")).toBe(5)
  })

  it("counts an astral code point as one char, not two UTF-16 units", () => {
    expect("🚀".length).toBe(2)
    expect(countChars("🚀")).toBe(1)
  })
})

describe("copyToastMessage", () => {
  it("pluralizes for many chars", () => {
    expect(copyToastMessage("hello")).toBe("copied 5 chars to clipboard")
  })

  it("uses the singular for exactly one char", () => {
    expect(copyToastMessage("a")).toBe("copied 1 char to clipboard")
  })
})
