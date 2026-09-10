# Crush

`crush` (`internal/providers.Registry`). It is xdg-basedir: config under `$XDG_CONFIG_HOME/crush` and data under
`$XDG_DATA_HOME/crush`. Its conversations are not there but in the checkout it was started in, as a database at
`.crush/crush.db`, so the same id proves nothing about another checkout. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Crush has no
per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

## Hooks

Crush reports no session state at all (the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)), so no turn ever opens or closes there. Two things
follow. A waiting card sends no reason and keeps the generic "Waiting on you"
(`frontend/src/components/sidebar/SessionCard.tsx`), which means the harness never spoke and not that the block
is trivial. And keep-awake has nothing to listen to (`internal/awake`, `turnLog.onOpen`): a Crush session left
working behind a locked screen sleeps as it always did, and no card says so.

## Transcript & cost

Nothing provider-specific.

## Plugin install

Crush has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). Crush records nothing
about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich reads
it as not installed. Crush below 0.88.0 ignores those lines in silence, which is why the install asks its version
first.

## MCP

Crush's config block registers lich's MCP server by the absolute path of the binary that installed it.

## Account

Crush runs on the user's own API keys, so there is no plan account to name (`internal/quota`, `Plan.Account`),
which is the same reason it has no gauge.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
