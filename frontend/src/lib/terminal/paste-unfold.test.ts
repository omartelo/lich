import { describe, expect, it } from "vitest"
import { pasteUnfold } from "./paste-unfold"

const ROWS = 40
const REPASTE = { kind: "repaste" }
const TAB = { kind: "key", key: "\t" }

function lines(count: number): string {
  return Array.from({ length: count }, (_, i) => `line ${i}`).join("\n")
}

describe("pasteUnfold for Claude Code", () => {
  it("pastes again above 800 characters", () => {
    expect(pasteUnfold("claude", "x".repeat(801), ROWS)).toEqual(REPASTE)
    expect(pasteUnfold("claude", "x".repeat(800), ROWS)).toBeNull()
  })

  it("pastes again above two newlines", () => {
    expect(pasteUnfold("claude", lines(4), ROWS)).toEqual(REPASTE)
    expect(pasteUnfold("claude", lines(3), ROWS)).toBeNull()
  })

  it("counts the way Claude Code does, after CRLF and tabs are normalized", () => {
    expect(pasteUnfold("claude", "a\r\nb\r\nc", ROWS)).toBeNull()
    expect(pasteUnfold("claude", "a\rb\rc\rd", ROWS)).toEqual(REPASTE)
    expect(pasteUnfold("claude", "\t".repeat(201), ROWS)).toEqual(REPASTE)
  })

  it("follows Claude Code's tighter newline limit on a short terminal", () => {
    expect(pasteUnfold("claude", "a\nb", 10)).toEqual(REPASTE)
    expect(pasteUnfold("claude", "a\nb", 11)).toBeNull()
    expect(pasteUnfold("claude", "a\nb", 5)).toEqual(REPASTE)
  })

  it("leaves a paste alone past the size Claude Code no longer unfolds", () => {
    expect(pasteUnfold("claude", "x".repeat(100_000), ROWS)).toEqual(REPASTE)
    expect(pasteUnfold("claude", "x".repeat(100_001), ROWS)).toBeNull()
  })
})

// The edges below are the ones measured against a real Kiro CLI 2.21.0.
describe("pasteUnfold for Kiro CLI", () => {
  it("presses Tab above 10 lines", () => {
    expect(pasteUnfold("kiro", lines(11), ROWS)).toEqual(TAB)
    expect(pasteUnfold("kiro", lines(10), ROWS)).toBeNull()
  })

  it("counts a trailing newline as the start of another line", () => {
    expect(pasteUnfold("kiro", `${lines(10)}\n`, ROWS)).toEqual(TAB)
  })

  it("counts a CRLF once, the way the terminal delivers it", () => {
    expect(pasteUnfold("kiro", lines(10).replace(/\n/g, "\r\n"), ROWS)).toBeNull()
  })

  it("presses Tab above 500 UTF-16 units", () => {
    expect(pasteUnfold("kiro", "x".repeat(501), ROWS)).toEqual(TAB)
    expect(pasteUnfold("kiro", "x".repeat(500), ROWS)).toBeNull()
    expect(pasteUnfold("kiro", "😀".repeat(251), ROWS)).toEqual(TAB)
    expect(pasteUnfold("kiro", "😀".repeat(250), ROWS)).toBeNull()
  })

  it("does not depend on the terminal's height", () => {
    expect(pasteUnfold("kiro", lines(4), 5)).toBeNull()
  })
})

describe("pasteUnfold elsewhere", () => {
  it("sends nothing after a paste into any other session", () => {
    for (const kind of ["shell", "codex", "opencode", "cursor"] as const) {
      expect(pasteUnfold(kind, lines(40), ROWS)).toBeNull()
    }
  })
})
