import { describe, expect, it } from "vitest"
import { pasteTimes } from "./paste-unfold"

const ROWS = 40

function lines(count: number): string {
  return Array.from({ length: count }, (_, i) => `line ${i}`).join("\n")
}

describe("pasteTimes", () => {
  it("pastes twice where Claude Code folds: more than 800 characters", () => {
    expect(pasteTimes("claude", "x".repeat(801), ROWS)).toBe(2)
    expect(pasteTimes("claude", "x".repeat(800), ROWS)).toBe(1)
  })

  it("pastes twice where Claude Code folds: more than two newlines", () => {
    expect(pasteTimes("claude", lines(4), ROWS)).toBe(2)
    expect(pasteTimes("claude", lines(3), ROWS)).toBe(1)
  })

  it("counts the way Claude Code does, after CRLF and tabs are normalized", () => {
    expect(pasteTimes("claude", "a\r\nb\r\nc", ROWS)).toBe(1)
    expect(pasteTimes("claude", "a\rb\rc\rd", ROWS)).toBe(2)
    expect(pasteTimes("claude", "\t".repeat(201), ROWS)).toBe(2)
  })

  it("follows Claude Code's tighter newline limit on a short terminal", () => {
    expect(pasteTimes("claude", "a\nb", 10)).toBe(2)
    expect(pasteTimes("claude", "a\nb", 11)).toBe(1)
    expect(pasteTimes("claude", "a\nb", 5)).toBe(2)
  })

  it("pastes once past the size Claude Code no longer unfolds", () => {
    expect(pasteTimes("claude", "x".repeat(100_000), ROWS)).toBe(2)
    expect(pasteTimes("claude", "x".repeat(100_001), ROWS)).toBe(1)
  })

  it("pastes once into every other session", () => {
    expect(pasteTimes("shell", lines(40), ROWS)).toBe(1)
    expect(pasteTimes("codex", lines(40), ROWS)).toBe(1)
  })
})
