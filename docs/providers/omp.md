# oh-my-pi

`omp` (`internal/providers.Registry`). Its state directory answers to two variables and the profile wins:
`OMP_PROFILE` moves the whole directory to `~/.omp/profiles/<profile>/agent` and beats an explicit
`PI_CODING_AGENT_DIR`, which otherwise names it, falling back to `~/.omp/agent`. One JSONL holds a conversation,
in a directory named after the cwd it ran in: `sessions/<encoded cwd>/<timestamp>_<id>.jsonl`. The limits these
sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` →
`relay.SpawnBriefing`): a session is spawned with `--append-system-prompt` carrying lich's own briefing, so text
the user never wrote is in every session's prompt and in `/proc/<pid>/cmdline`.

## Hooks

An oh-my-pi card sends no reason with a `waiting` and keeps the generic "Waiting on you"
(`frontend/src/components/sidebar/SessionCard.tsx`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). It does not report `waiting` in the first place: it
declares an approval event no run was ever seen emitting, so there is nothing to hang a reason on. A bare card
there means the harness never spoke, not that the block is trivial.

## Transcript & cost

Nothing provider-specific.

## Plugin install

oh-my-pi has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). omp records
nothing about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich
reads it as not installed.

Both halves land in the state directory above, resolved independently by the install and by the transcript
reader (`internal/agentplugin/omp.go`, `internal/terminal/transcript.go`, as the Claude Code pair do). Get the
precedence backwards and the install lands where omp is not reading and every restored card silently starts
fresh.

## MCP

omp's `mcp.json` registers lich's MCP server by the absolute path of the binary that installed it, and it is a
JSON document lich rewrites rather than appends to: every key survives, the user's formatting does not.

## Account

oh-my-pi runs on the user's own API keys, so there is no plan account to name (`internal/quota`, `Plan.Account`),
which is the same reason it has no gauge.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
