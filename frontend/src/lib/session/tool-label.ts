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

// toolLabel shortens what it can prove and leaves the rest alone.
//
// The doubled form splits cleanly, so it draws as "<server> · <tool>" —
// non-greedy on the server, so a tool whose own name contains `__` keeps it.
// omp's single underscore cannot be split at all: `mcp__lich_list_sessions`
// divides into "lich" + "list_sessions" or "lich_list" + "sessions" and nothing
// in the string says which, so only the prefix comes off. opencode's form has no
// marker to key on — a server name is not distinguishable from the first word of
// a tool name — so it is shown as it arrived, which is also what any name that is
// not an MCP tool's gets.
export function toolLabel(name: string): string {
  const parts = MCP_DOUBLE.exec(name)
  if (parts) {
    return `${parts[1]} · ${parts[2]}`
  }
  return name.startsWith(MCP_PREFIX) ? name.slice(MCP_PREFIX.length) : name
}
