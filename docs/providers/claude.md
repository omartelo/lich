# Claude Code

`claude` (`internal/providers.Registry`), the default provider and the companion plugin's home. Its config
directory is `$CLAUDE_CONFIG_DIR`, else `~/.claude`, with the account in `~/.claude.json` beside it. A
conversation is one JSONL at `projects/<slug>/<id>.jsonl`, and a sub-agent's is a file of its own under
`projects/<slug>/<id>/subagents/`. The limits these sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` →
`relay.SpawnBriefing`): a session is spawned with `--append-system-prompt` carrying lich's own briefing, so text
the user never wrote is in every session's prompt and in `/proc/<pid>/cmdline`.

## Hooks

Claude Code is the only provider that says what a session is waiting for
(`frontend/src/components/sidebar/SessionCard.tsx`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). Its `Notification` carries a `message` written for a
human, so the card reads "Claude needs your permission to use Bash".

## Transcript & cost

Nothing provider-specific.

## Plugin install

Driven through Claude Code's own plugin CLI (`internal/agentplugin`), so lich writes none of the files itself
and does not track the installed version in a marker line of its own.

## MCP

The session is handed lich's own MCP server on its command line (`--mcp-config`,
`providers.AcceptsMCPServer`), so a tool added here is in that session's list on the next spawn.

## Account

The plan gauge names the account a session spends (`internal/quota`, `Plan.Account`): lich answers Claude
Code's profile route with the credentials token.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
