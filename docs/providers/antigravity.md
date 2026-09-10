# Antigravity

`agy` (`internal/providers.Registry`). Its state lives under `~/.gemini`, read off the home alone: 1.1.19 falls
back to a hardcoded `.gemini` when it cannot resolve the home and honours no environment variable of its own.
The OAuth credentials sit at that root, the customizations under `config/`, and a conversation is SQLite at
`antigravity-cli/conversations/<id>.db` with its JSONL beside it under
`antigravity-cli/brain/<id>/.system_generated/logs/`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Antigravity
has no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

## Hooks

An Antigravity card sends no reason with a `waiting` and keeps the generic "Waiting on you"
(`frontend/src/components/sidebar/SessionCard.tsx`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). It does not report `waiting` in the first place: its
permission prompt raises no lifecycle event that has been measured, so there is nothing to hang a reason on. A
bare card there means the harness never spoke, not that the block is trivial.

## Transcript & cost

Nothing provider-specific.

## Plugin install

Antigravity has a plugin CLI and lich installs around it (`internal/agentplugin/antigravity.go`): `agy plugin
install` takes a directory, and its only remote form clones that repository's default branch, while lich
installs a *release*, whose version is what a card reports and what the next update compares against. So lich
writes the customization directory itself (`~/.gemini/config/plugins/lich/`), with three consequences. The
installed version lives in the manifest lich writes rather than in a marker line, since the directory is lich's
outright, and a copy the user installed through `agy plugin install` carries no version, so it reads as not
installed. The registration's commands are relative, resolved against the directory holding `hooks.json`
(Antigravity runs a hook through `sh -c` from there and sets no plugin-root variable of its own, both measured
on 1.1.19), so moving that directory by hand breaks every report until the next install. And lich writes the
hooks and their scripts only: the plugin's skills come with `agy plugin install`, not with this, which is the
same thing already true of opencode, oh-my-pi and Crush.

## MCP

Nothing provider-specific.

## Account

`~/.gemini/oauth_creds.json` carries the same `email` claim Codex's id token does, and it is deliberately not
read (`internal/quota`, `Plan.Account`): there is no `antigravityPlan`, so a `Plan` naming an account with no
window in it renders nothing at all (`PlanQuota` draws only a reading with a window). The name arrives with the
gauge or not at all.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
