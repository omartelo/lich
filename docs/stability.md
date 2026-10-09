# Stability promise

lich follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). This page says which parts of lich that
promise covers, so you know what is safe to build scripts, plugins and themes on.

## What is covered

A breaking change to anything in this section ships in a new major version, or goes through a deprecation first: the
old form keeps working, with a visible warning, for at least one minor release before it is removed.

- **The `lich` CLI**, except the experimental commands below. Subcommands, flags, the field names and meaning of
  `--json` output, the columns of `--csv` output, and exit codes. [cli.md](cli.md) is the reference for each of them.
- **MCP tools**, except the experimental ones below. The tool names and input schemas that `lich mcp` serves
  (`internal/cli/mcp.go`).
- **The hook contract** with [`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin), except the parts
  listed as experimental below: the endpoints and payloads written down in [hooks/](hooks/README.md). Which plugin
  releases a lich supports, and how the plugin's version number marks a contract change, is under [Versioning](hooks/README.md#versioning).
- **Themes.** The theme JSON format and the `lich-theme.json` pack manifest, as described in [themes.md](themes.md).
- **Project files.** `.lich/setup-worktree.sh`, `.lich/run-worktree.sh` and `.worktreeinclude`.
- **Environment variables lich sets in every session:** `LICH_PORT`, `LICH_TOKEN`, `LICH_SESSION_ID`, `LICH_BIN`,
  `LICH_PROJECT_DIR` and `LICH_WORKTREE_PORT`.
- **Environment variables lich reads at startup:** `LICH_SHELL`, `LICH_LISTEN_PORT` and `LICH_LOG_LEVEL`.

## What is experimental

These work, but they rest on another product's undocumented internals, which lich does not control. They may change
or stop working in any release, minor or patch, outside semver, and lich marks them where you meet them: in
`lich help`, in [cli.md](cli.md) and in the MCP tool descriptions. Build on them knowing that.

- **`lich control` and `lich ask`**, and the MCP tools `control_session` and `ask_session`. Claude Code only: they
  run through lich-plugin's mod inside the Claude Code process, and an ask is a fork of the session's conversation
  made from there. What they can do, and the traps listed in
  [mod-control.md](hooks/mod-control.md#known-ceilings), are behaviour measured on specific Claude Code releases,
  not a contract Claude Code publishes.
- **A Claude Code subagent opening as a lich card**, and `lich open --subagent`, which it runs on: lich-plugin's mod
  takes Claude Code's own `Agent` call, and the worker's report comes back to the asking session as a Claude Code
  task notification, through a render path Claude Code does not document (see
  [ceilings.md](ceilings.md)). A Claude Code that drops that path shows the report as a typed prompt instead.
- **The hook contract parts that carry those features:** `/mod/commands` ([mod-control.md](hooks/mod-control.md))
  and `/mod/answer` ([mod-answer.md](hooks/mod-answer.md)). Their shape follows what the features above need from
  Claude Code, so it moves with them. The relayed tasks and notes `/mod/commands` also carries fall back to being
  typed at the terminal, so `lich send` and `send_to_session` stay covered. `/mod/status` and `/mod/usage` stay
  covered too: one reads lich's own errands, the other reports the figures Claude Code hands its status line, and
  neither depends on how Claude Code forks or renders a conversation.

## What is internal

These can change in any release, minor or patch, without notice. Do not build on them.

- The HTTP endpoints the window talks to: `/rpc`, `/ws`, `/events`, `/restart` and `/debug`. A script that needs
  something from a running lich calls the `lich` CLI, which is covered.
- `runtime.json`.
- The SQLite database schema and the names of settings keys.
- The files lich writes into a provider's own config directory. lich rewrites them itself, so hand edits there do
  not last.
- The log format.

Internal does not mean your data is at risk. Your sessions, projects and settings are migrated forward on upgrade;
only the shape they are stored in is not a contract.

## Upgrades and downgrades

Upgrading from any earlier release to a newer one is supported, and lich migrates what it stores on first start.
Downgrading is not supported: an older lich may not read what a newer one wrote.
