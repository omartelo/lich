# Cursor CLI

`cursor-agent` (`internal/providers.Registry`). Cursor keeps its state in two directories. Its config dir is
`$CURSOR_CONFIG_DIR` ‖ `$XDG_CONFIG_HOME/cursor` ‖ `~/.cursor`, not xdg-basedir, the fallback being the home
directly, and it holds the credentials and the chats. But `~/.cursor` is resolved off the home with no variable
in the way at all, and that is where `mcp.json`, the per-project transcripts and the CLI state live. The chat
itself is SQLite at `chats/<md5 of the resolved cwd>/<chatId>/store.db`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): the CLI has
no append flag, so what a Cursor session knows about lich is its tool list.

## Hooks

A Cursor CLI session reports through Claude Code's plugin, or not at all (`internal/agentplugin`,
`internal/terminal/start.go`, `providerKind`). lich installs no plugin into Cursor, and it does not have to: the
CLI executes every Claude Code hook on the machine, the user's own and each installed plugin's (measured on
2026.08.11: `hookSource: claude-user` and `claude-plugin`, with `${CLAUDE_PLUGIN_ROOT}` expanded). So on a
machine where the lich plugin is installed in Claude Code, a Cursor session reports the chat id it is running and
the files it touches, with nothing installed there, and on a machine without it that session reports nothing at
all. Nothing on the card says which of the two it is.

**What never arrives is the turn.** Of the nine events the plugin registers, Cursor delivers four,
`SessionStart`, `PreToolUse`, `PostToolUse` and `SessionEnd`, and no `UserPromptSubmit` or `Stop`, measured
against hooks in Cursor's own format and in Claude Code's alike. So a turn that calls no tool never begins and
one that does never ends, which is why `terminal.closableState` drops every state but `idle` from a Cursor
session rather than pinning a spinner to the card for the rest of it: no spinner, no bell, no auto-title, and no
`waiting` either (`Notification` maps to nothing there). That is the Crush row of
[`../hooks/session-state.md`](../hooks/session-state.md)'s table arrived at from the other direction, since lich
does not own the registration here and so filters what it cannot close.

## Transcript & cost

Cursor keeps its chats per checkout, so a resume asked without the session's own working directory answers
"conversation gone" (`internal/terminal/transcript.go`), the same shape as Crush.

## Plugin install

lich installs nothing into Cursor. Installing for Cursor refuses while Claude Code has no plugin, its version is
Claude Code's, and its row offers no update of its own: the update is the Claude Code row's, one line up the same
screen.

## MCP

Cursor takes no MCP server on its command line and reads none from a Claude Code plugin, so lich's own tools come
from an `mcpServers` document its install writes under `~/.cursor`.

## Account

Cursor CLI carries no claim naming the account, measured on 2026-09-04 (`internal/quota`, `Plan.Account`):
`~/.config/cursor/auth.json`'s token is a JWT whose `sub` is an opaque `github|user_…` and whose claims contain
no email, so there is nothing a person recognises as their account. Naming it would take a network call against
an unmeasured route, for a provider that has no gauge to hang the name under.

## Sandbox

The two directories above are two mounts (`internal/sandbox/sandbox.go`). On a machine with `XDG_CONFIG_HOME` set
they are different directories, and a sandbox binding only one is a session that cannot see its own MCP servers.

## Known limits

The plugin's script reports `claude`, the argument Claude Code's own registration passes it, and lich drops that
name for a card whose provider it chose itself. A **shell** session running `cursor-agent` by hand has only the
report to go on and wears Claude's mark.
