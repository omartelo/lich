// Claude Code and Codex both spell an MCP tool `mcp__<server>__<tool>`
// (docs/hooks/session-state.md). The prefix and the doubled underscores are
// machinery — they cost a card its whole width before the part worth reading
// starts, and a session card is 12rem at its narrowest. The server and the tool
// are the news, so the label keeps those and drops the rest.
//
// Non-greedy on the server so a tool whose own name contains `__` keeps it: the
// first `__` after the prefix ends the server, and everything after it is the
// tool. A name that is not an MCP tool's is shown as it arrived — the card
// promises to show whatever a harness reports.
const MCP_TOOL = /^mcp__(.+?)__(.+)$/

export function toolLabel(name: string): string {
  const parts = MCP_TOOL.exec(name)
  return parts ? `${parts[1]} · ${parts[2]}` : name
}
