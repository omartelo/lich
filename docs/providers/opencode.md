# opencode

`opencode` (`internal/providers.Registry`). It reads its directories through the xdg-basedir convention on every
platform rather than the OS-native ones: config under `$XDG_CONFIG_HOME/opencode`, and every conversation in one
database at `$XDG_DATA_HOME/opencode/opencode.db`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): opencode has
no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

opencode files a fork under the parent's directory, not the new checkout's (measured on 1.18.23): the copied
session's `directory` column is the one the original ran in, and its `parent_id` is left empty, so opencode's own
session list places a fork in the checkout it came from and records no lineage. lich's own card is right, since
it carries the worktree it was opened in and `origin_session_id` names the parent, but the two disagree and only
lich's side is visible in lich.

## Hooks

opencode's `.asked` events carry only the thing being asked about (`permission`, `action`), so a waiting card
reads a bare `edit` (`frontend/src/components/sidebar/SessionCard.tsx`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). That says which card to open and not what it will ask.

## Transcript & cost

Nothing provider-specific.

## Plugin install

opencode has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). opencode records
nothing about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich
reads it as not installed.

## MCP

A new MCP tool reaches opencode a release later than everyone else (`internal/cli/mcp.go`, `mcpTools`; the
registration table in [`../cli.md`](../cli.md)). Every other harness is handed lich's own server, so a tool added
there is in that session's list on the next spawn. opencode cannot register an MCP server from a plugin, so its
plugin defines each tool itself in the companion repo (`omartelo/lich-plugin`, `opencode/lich.js`), which means a
tool arrives there only once that repo cuts a release and the user reinstalls the plugin. Until they do it is
missing from that session's list while it is in every other. `lich rename` works there like anywhere else; it is
discovery that lags, which is the whole reason the tools exist.

## Account

opencode runs on the user's own API keys, so there is no plan account to name (`internal/quota`, `Plan.Account`),
which is the same reason it has no gauge.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
