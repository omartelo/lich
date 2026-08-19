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

  // Measured against omp 17.3.7: a single underscore between the server and the
  // tool, which nothing in the string can split — "lich" + "list_sessions" and
  // "lich_list" + "sessions" are equally readable, so only the prefix comes off.
  it("takes the prefix off omp's single-underscore form and splits no further", () => {
    expect(toolLabel("mcp__lich_list_sessions")).toBe("lich_list_sessions")
  })

  // Measured against opencode 1.18.18: no prefix at all, so there is no marker
  // to key on and the name is shown whole.
  it("leaves opencode's form alone", () => {
    expect(toolLabel("lichprobe_list_sessions")).toBe("lichprobe_list_sessions")
  })

  it("shows a name that is not an MCP tool's exactly as it arrived", () => {
    expect(toolLabel("Bash")).toBe("Bash")
    expect(toolLabel("apply_patch")).toBe("apply_patch")
    expect(toolLabel("")).toBe("")
  })

  // Half a prefix is not an MCP name, and neither half is a server or a tool.
  it("still strips the prefix off a name it cannot split", () => {
    expect(toolLabel("mcp__lich")).toBe("lich")
    expect(toolLabel("mcp____tool")).toBe("__tool")
  })
})
