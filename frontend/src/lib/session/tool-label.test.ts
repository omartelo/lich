import { describe, expect, it } from "vitest"
import { toolLabel } from "./tool-label"

describe("toolLabel", () => {
  it("drops the mcp machinery and keeps the server and the tool", () => {
    expect(toolLabel("mcp__ai-memory__memory_write_page")).toBe("ai-memory · memory_write_page")
    expect(toolLabel("mcp__lich__open_session")).toBe("lich · open_session")
  })

  // The server ends at the first `__` after the prefix, so a tool whose own
  // name carries one keeps it whole rather than losing its tail to the split.
  it("splits on the first separator, not the last", () => {
    expect(toolLabel("mcp__srv__do__the__thing")).toBe("srv · do__the__thing")
  })

  it("shows a name that is not an MCP tool's exactly as it arrived", () => {
    expect(toolLabel("Bash")).toBe("Bash")
    expect(toolLabel("apply_patch")).toBe("apply_patch")
    expect(toolLabel("")).toBe("")
  })

  // Half a prefix is not an MCP name, and neither half is a server or a tool.
  it("leaves a malformed mcp name alone", () => {
    expect(toolLabel("mcp__lich")).toBe("mcp__lich")
    expect(toolLabel("mcp____tool")).toBe("mcp____tool")
  })
})
