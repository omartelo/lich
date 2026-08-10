import { describe, expect, it } from "vitest"
import { toolGlyph } from "./tool-glyph"

describe("toolGlyph", () => {
  it("gives the same glyph to both harnesses' word for one action", () => {
    // Codex reports an edit as apply_patch and everything else under the names
    // Claude Code uses (docs/hooks/session-state.md).
    expect(toolGlyph("Edit")).toBe(toolGlyph("apply_patch"))
    expect(toolGlyph("Write")).toBe(toolGlyph("apply_patch"))
    expect(toolGlyph("Grep")).toBe(toolGlyph("Glob"))
  })

  it("has no glyph for a tool it does not know", () => {
    expect(toolGlyph("mcp__ai-memory__memory_query")).toBeNull()
    expect(toolGlyph("")).toBeNull()
  })

  // The name arrives over the wire. An object literal would answer this one
  // with Object.prototype.constructor, which React would then try to render.
  it("has no glyph for an inherited object property", () => {
    expect(toolGlyph("constructor")).toBeNull()
    expect(toolGlyph("toString")).toBeNull()
    expect(toolGlyph("__proto__")).toBeNull()
  })
})
