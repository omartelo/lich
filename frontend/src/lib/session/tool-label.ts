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
export function toolLabel(name: string, servers: readonly string[] = []): string {
  const parts = MCP_DOUBLE.exec(name)
  if (parts) {
    return `${parts[1]} · ${parts[2]}`
  }
  const rest = name.startsWith(MCP_PREFIX) ? name.slice(MCP_PREFIX.length) : name
  const server = longestServer(rest, servers)
  return server ? `${server} · ${rest.slice(server.length + 1)}` : rest
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
