import { describe, expect, it } from "vitest"
import { toolLabel } from "./tool-label"

// The names a session's provider reports its servers under (providers.Detect).
const SERVERS = ["lich", "ai-memory"]

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
  // tool, which only the list of registered servers can divide.
  it("splits omp's single-underscore form against the servers", () => {
    expect(toolLabel("mcp__lich_list_sessions", SERVERS)).toBe("lich · list_sessions")
    expect(toolLabel("mcp__ai-memory_memory_query", SERVERS)).toBe("ai-memory · memory_query")
  })

  // Measured against opencode 1.18.18: no prefix at all, so the servers are the
  // whole of what says where the name divides.
  it("splits opencode's bare form against the servers", () => {
    expect(toolLabel("lich_open_session", SERVERS)).toBe("lich · open_session")
  })

  // Two servers could claim the same name; the longer one is the one that read
  // the whole prefix rather than stopping inside it.
  it("takes the longest server that matches", () => {
    expect(toolLabel("lich_relay_send", ["lich", "lich_relay"])).toBe("lich_relay · send")
  })

  // A server name is not a word boundary: `lichen_pick` is not lich's.
  it("matches a whole server name, not a prefix of one", () => {
    expect(toolLabel("lichen_pick", SERVERS)).toBe("lichen_pick")
  })

  it("leaves a name no server claims exactly as it arrived", () => {
    expect(toolLabel("lichprobe_list_sessions", SERVERS)).toBe("lichprobe_list_sessions")
    expect(toolLabel("mcp__other_list_sessions", SERVERS)).toBe("other_list_sessions")
    expect(toolLabel("mcp__lich_list_sessions")).toBe("lich_list_sessions")
  })

  it("shows a name that is not an MCP tool's exactly as it arrived", () => {
    expect(toolLabel("Bash", SERVERS)).toBe("Bash")
    expect(toolLabel("apply_patch", SERVERS)).toBe("apply_patch")
    expect(toolLabel("", SERVERS)).toBe("")
  })

  // Half a prefix is not an MCP name, and neither half is a server or a tool.
  it("still strips the prefix off a name it cannot split", () => {
    expect(toolLabel("mcp__lich", SERVERS)).toBe("lich")
    expect(toolLabel("mcp____tool", SERVERS)).toBe("__tool")
    // A server with nothing after it draws no separator.
    expect(toolLabel("lich_", SERVERS)).toBe("lich_")
  })
})
