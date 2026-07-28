import { describe, expect, it } from "vitest"
import { parseBoolPref, parseEnumPref, parseNumberPref } from "./prefs"

const THEMES = ["system", "light", "dark"] as const

describe("parseEnumPref", () => {
  it("keeps a value the build knows", () => {
    expect(parseEnumPref("dark", THEMES, "system")).toBe("dark")
  })

  it("falls back for an absent key", () => {
    expect(parseEnumPref(null, THEMES, "system")).toBe("system")
  })

  it("falls back for a value from another build", () => {
    expect(parseEnumPref("solarized", THEMES, "system")).toBe("system")
  })

  it("does not accept a value that only looks like one", () => {
    expect(parseEnumPref("Dark", THEMES, "system")).toBe("system")
    expect(parseEnumPref("", THEMES, "system")).toBe("system")
  })
})

describe("parseNumberPref", () => {
  const clamp = (value: number) => Math.min(32, Math.max(8, Math.round(value)))

  it("clamps a stored value into this build's bounds", () => {
    expect(parseNumberPref("14", 14, clamp)).toBe(14)
    expect(parseNumberPref("999", 14, clamp)).toBe(32)
    expect(parseNumberPref("2", 14, clamp)).toBe(8)
  })

  it("falls back for an absent key", () => {
    expect(parseNumberPref(null, 14, clamp)).toBe(14)
  })

  it("falls back for anything that is not a number", () => {
    expect(parseNumberPref("large", 14, clamp)).toBe(14)
    expect(parseNumberPref("", 14, clamp)).toBe(14)
    expect(parseNumberPref("Infinity", 14, clamp)).toBe(14)
  })

  // A magnitude of zero or less is garbage, not a small setting: clamping it to
  // the minimum would silently accept it.
  it("falls back rather than clamping a non-positive value", () => {
    expect(parseNumberPref("0", 14, clamp)).toBe(14)
    expect(parseNumberPref("-3", 14, clamp)).toBe(14)
  })
})

describe("parseBoolPref", () => {
  it("reads what writePref emits", () => {
    expect(parseBoolPref("true", false)).toBe(true)
    expect(parseBoolPref("false", true)).toBe(false)
  })

  it("falls back for an absent key", () => {
    expect(parseBoolPref(null, true)).toBe(true)
    expect(parseBoolPref(null, false)).toBe(false)
  })

  it("falls back for a value it did not write", () => {
    expect(parseBoolPref("1", true)).toBe(true)
    expect(parseBoolPref("1", false)).toBe(false)
    expect(parseBoolPref("yes", false)).toBe(false)
  })
})
