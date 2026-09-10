import { describe, expect, it } from "vitest"
import { toolLine } from "./tool-label"

// The names a session's provider reports its servers under (providers.Detect).
const SERVERS = ["lich", "ai-memory"]

// Most of what this file pins is the name half alone: a tool whose detail is
// its own words about the call, which the line carries through untouched.
const label = (name: string, servers: readonly string[] = []) =>
  toolLine({ name, detail: "" }, servers).label

describe("toolLine", () => {
  it("drops the mcp machinery and keeps the server and the tool", () => {
    expect(label("mcp__ai-memory__memory_write_page")).toBe("ai-memory · memory_write_page")
    expect(label("mcp__lich__open_session")).toBe("lich · open_session")
  })

  // The server ends at the first `__` after the prefix, so a tool whose own
  // name carries one keeps it whole rather than losing its tail to the split.
  it("splits on the first separator, not the last", () => {
    expect(label("mcp__srv__do__the__thing")).toBe("srv · do__the__thing")
  })

  // Measured against omp 17.3.7: a single underscore between the server and the
  // tool, which only the list of registered servers can divide.
  it("splits omp's single-underscore form against the servers", () => {
    expect(label("mcp__lich_list_sessions", SERVERS)).toBe("lich · list_sessions")
    expect(label("mcp__ai-memory_memory_query", SERVERS)).toBe("ai-memory · memory_query")
  })

  // Measured against opencode 1.18.18: no prefix at all, so the servers are the
  // whole of what says where the name divides.
  it("splits opencode's bare form against the servers", () => {
    expect(label("lich_open_session", SERVERS)).toBe("lich · open_session")
  })

  // Two servers could claim the same name; the longer one is the one that read
  // the whole prefix rather than stopping inside it.
  it("takes the longest server that matches", () => {
    expect(label("lich_relay_send", ["lich", "lich_relay"])).toBe("lich_relay · send")
  })

  // A server name is not a word boundary: `lichen_pick` is not lich's.
  it("matches a whole server name, not a prefix of one", () => {
    expect(label("lichen_pick", SERVERS)).toBe("lichen_pick")
  })

  it("leaves a name no server claims exactly as it arrived", () => {
    expect(label("lichprobe_list_sessions", SERVERS)).toBe("lichprobe_list_sessions")
    expect(label("mcp__other_list_sessions", SERVERS)).toBe("other_list_sessions")
    expect(label("mcp__lich_list_sessions")).toBe("lich_list_sessions")
  })

  it("shows a name that is not an MCP tool's exactly as it arrived", () => {
    expect(label("Bash", SERVERS)).toBe("Bash")
    expect(label("apply_patch", SERVERS)).toBe("apply_patch")
    expect(label("", SERVERS)).toBe("")
  })

  // Half a prefix is not an MCP name, and neither half is a server or a tool.
  it("still strips the prefix off a name it cannot split", () => {
    expect(label("mcp__lich", SERVERS)).toBe("lich")
    expect(label("mcp____tool", SERVERS)).toBe("__tool")
    // A server with nothing after it draws no separator.
    expect(label("lich_", SERVERS)).toBe("lich_")
  })

  it("keeps the detail the harness sent beside the name", () => {
    expect(toolLine({ name: "Bash", detail: "pnpm test" }, SERVERS)).toEqual({
      label: "Bash",
      detail: "pnpm test",
    })
  })

  // Measured against Antigravity 1.1.19: the step is the same for every MCP
  // call, and the plugin sends the tool it stands for as the detail. Splitting
  // that is what makes the line read like every other provider's.
  it("splits Antigravity's step against its detail", () => {
    expect(toolLine({ name: "call_mcp_tool", detail: "lich/open_session" }, SERVERS)).toEqual({
      label: "lich · open_session",
      detail: "",
    })
  })

  // A plugin older than the release that added the detail sends the step alone.
  it("draws the step whole when it carries no detail", () => {
    expect(toolLine({ name: "call_mcp_tool", detail: "" }, SERVERS)).toEqual({
      label: "call_mcp_tool",
      detail: "",
    })
  })

  // Not every detail is an identity: one that does not spell `<server>/<tool>`
  // is the harness's own words, and both halves stay as they arrived.
  it("leaves a detail that is not a server and a tool alone", () => {
    expect(toolLine({ name: "call_mcp_tool", detail: "open_session" }, SERVERS)).toEqual({
      label: "call_mcp_tool",
      detail: "open_session",
    })
    expect(toolLine({ name: "call_mcp_tool", detail: "lich/" }, SERVERS)).toEqual({
      label: "call_mcp_tool",
      detail: "lich/",
    })
  })
})
