# Kiro CLI

`kiro-cli` (`internal/providers.Registry`), spawned as `kiro-cli chat`, since `--model` and `--trust-all-tools`
are the `chat` subcommand's while `--agent` and `--resume-id` are the root's. `~/.kiro` hangs off the home alone,
with no environment variable to honour (2.21.0 resolves `$HOME/.kiro`), and holds the agents, the settings and
the conversations; the login token is a row in the SQLite store under `$XDG_DATA_HOME/kiro-cli/data.sqlite3`. A
conversation's metadata is `~/.kiro/sessions/cli/<id>.json` with its turns in the `.jsonl` beside it. The limits
these sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

**A lich-spawned Kiro session runs lich's agent, not the user's** (`internal/agentplugin/kiro.go`,
`internal/terminal/command.go`, `agentArgs`). Kiro keeps its hooks inside an *agent config*, and its built-in
`kiro_default` cannot be shadowed: a `kiro_default.json` in the agents directory is ignored outright (measured on
2.21.0). So the only place a hook can be registered is an agent lich writes itself, which the spawn then names
with `--agent lich`. Two things follow, and neither is visible on the card. The agent lich writes carries no
`prompt`, so a session loses the paragraph `kiro_default` adds about subagents, the planner and LSP: the model
still has every tool, it is just not told about those. And a default the user set with
`kiro-cli agent set-default` is **not** what a lich session runs; their own agent applies to Kiro started from a
terminal and not to Kiro started from a card. Deleting `~/.kiro/agents/lich.json` puts the session back on `kiro_default` and
silently ends every report with it: the spawn stops naming an agent it cannot find, which is the one case where
that costs a warning line rather than the reports.

Kiro's skip-permissions flag still stops once (`internal/terminal/command.go`, `skipPermissionFlags`). With the
switch on, lich passes `--trust-all-tools`, and Kiro's TUI opens on a full-screen confirmation ("No, exit / Yes,
I accept / Yes, and don't ask again") that has to be answered before the session starts. The flag is real and
every tool afterwards runs unconfirmed; it is the *first* screen that is not skipped, which is the one thing a
user who ticked the box does not expect. Kiro's own `chat.disableTrustAllConfirmation` setting turns it off for
good, and lich does not write it: that setting disables a safety confirmation for every Kiro on the machine,
including the ones lich never spawned.

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` has no entry
for Kiro), so the point the briefing makes exists only in lich's MCP instructions.

## Hooks

**Kiro enforces no hook timeout, and blocks the turn behind one** (`internal/agentplugin/kiro.go`): its TUI draws
"0 of 1 hooks finished" while a report runs, and a hook that slept four seconds ran to completion under both
`timeout_ms: 1000` and `timeout: 1` (2.21.0). So lich writes no timeout into the agent, a field that bounds
nothing being a promise the file does not keep, and what actually bounds a report is the request timeout inside
the script itself. A lich whose listener has gone away costs a Kiro turn that wait on every hook it fires, where
the other harnesses cut it off themselves.

**A Kiro session never reports `waiting` or `idle`** ([`../hooks/`](../hooks/)). Its five events are `agentSpawn`,
`userPromptSubmit`, `preToolUse`, `postToolUse` and `stop`, so lich closes session-start, session-state's
busy/tool/done rows and session-touched, and nothing else. A Kiro session sitting on a permission prompt reads as
`busy`, true, but it does not say what it is waiting for, and the card keeps the provider's mark until the PTY
itself goes, because there is no session-end event to clear it. That `busy` also holds the machine awake
(`internal/awake`, `turnLog.onOpen`): a Kiro session blocked on a human keeps it out of idle sleep until someone
answers.

## Transcript & cost

Nothing provider-specific.

## Plugin install

The hooks are the agent config named under Spawn, and there is nothing else to install: Kiro has no plugin
system, so the version lives in that file's `description` rather than in a marker line.

## MCP

Nothing provider-specific.

## Account

Kiro CLI carries no claim naming the account, measured on 2026-09-04 (`internal/quota`, `Plan.Account`): its
login is a row in `~/.local/share/kiro-cli/data.sqlite3` holding an opaque `aoa…` token, a `github` provider
label and an AWS profile ARN, nothing a person recognises as their account. Naming it would take a network call
against an unmeasured route, for a provider that has no gauge to hang the name under.

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
