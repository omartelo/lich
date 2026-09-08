import type { SessionTool } from "./session-events"

// The harnesses spell an MCP tool three different ways, all measured against the
// real CLIs rather than read off their docs (docs/hooks/session-state.md):
//
//   Claude Code, Codex  mcp__<server>__<tool>   mcp__lich__open_session
//   oh-my-pi            mcp__<server>_<tool>    mcp__lich_list_sessions
//   opencode            <server>_<tool>         lichprobe_list_sessions
//
// The `mcp__` prefix and the doubled underscores are machinery: they cost a card
// its width before the part worth reading starts, and a session card is 12rem at
// its narrowest.
const MCP_DOUBLE = /^mcp__(.+?)__(.+)$/
const MCP_PREFIX = "mcp__"

// Antigravity is the one harness whose tool name is not the tool: every MCP call
// there is the single step `call_mcp_tool`, and which server and which tool are
// two of its arguments, which the plugin reads and sends as the report's detail
// (`<server>/<tool>`). So the identity to split is in the other field, and a
// server name never carries a slash.
const MCP_STEP = "call_mcp_tool"
const MCP_STEP_DETAIL = /^([^/]+)\/(.+)$/

export interface ToolLine {
  label: string
  detail: string
}

// toolLabel shortens what it can prove and leaves the rest alone, drawing an MCP
// tool as "<server> · <tool>".
//
// The doubled form splits on the string alone — non-greedy on the server, so a
// tool whose own name contains `__` keeps it. The single-underscore forms cannot:
// `mcp__lich_list_sessions` divides into "lich" + "list_sessions" or "lich_list"
// + "sessions" and nothing in the string says which. What says which is servers,
// the names that session's own spawn found registered with its provider —
// matched longest first, so a `lich_relay` server wins over a `lich` one on a
// name both could claim.
//
// A name matching no server keeps whatever survives the prefix, which is also
// what any name that is not an MCP tool's gets, and what every name got before
// a session had a list.
function toolLabel(name: string, servers: readonly string[] = []): string {
  const parts = MCP_DOUBLE.exec(name)
  if (parts) {
    return `${parts[1]} · ${parts[2]}`
  }
  const rest = name.startsWith(MCP_PREFIX) ? name.slice(MCP_PREFIX.length) : name
  const server = longestServer(rest, servers)
  return server ? `${server} · ${rest.slice(server.length + 1)}` : rest
}

// toolLine is the two halves a card draws for the tool a turn is running: the
// label, and whatever the harness sent to identify the call beside it. The
// single entry point on purpose: a card that reached for the label alone drew
// Antigravity's step name with the tool it stands for repeated after it
// (`call_mcp_tool · lich/open_session`), where every other provider drew
// `lich · open_session`.
//
// A step whose detail is not `<server>/<tool>` keeps both halves as they
// arrived, which is also what a plugin too old to send that detail gets: the
// step name whole, the way any tool that cannot be split already looks.
export function toolLine(tool: SessionTool, servers: readonly string[] = []): ToolLine {
  const step = tool.name === MCP_STEP ? MCP_STEP_DETAIL.exec(tool.detail) : null
  if (step) {
    return { label: `${step[1]} · ${step[2]}`, detail: "" }
  }
  return { label: toolLabel(tool.name, servers), detail: tool.detail }
}

// longestServer is the longest name in servers that rest spells as `<name>_`,
// or "" when none does — and never one with nothing after it, which would draw a
// separator with an empty tool beside it.
function longestServer(rest: string, servers: readonly string[]): string {
  let found = ""
  for (const server of servers) {
    if (server.length > found.length && rest.length > server.length + 1) {
      if (rest.startsWith(`${server}_`)) found = server
    }
  }
  return found
}
