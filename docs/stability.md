# Stability promise

lich follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). This page says which parts of lich that
promise covers, so you know what is safe to build scripts, plugins and themes on.

## What is covered

A breaking change to anything in this section ships in a new major version, or goes through a deprecation first: the
old form keeps working, with a visible warning, for at least one minor release before it is removed.

- **The `lich` CLI.** Subcommands, flags, the field names and meaning of `--json` output, the columns of `--csv`
  output, and exit codes. [cli.md](cli.md) is the reference for each of them.
- **MCP tools.** The tool names and input schemas that `lich mcp` serves (`internal/cli/mcp.go`).
- **The hook contract** with [`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin): the endpoints and
  payloads written down in [hooks/](hooks/README.md).
- **Themes.** The theme JSON format and the `lich-theme.json` pack manifest, as described in [themes.md](themes.md).
- **Project files.** `.lich/setup-worktree.sh`, `.lich/run-worktree.sh` and `.worktreeinclude`.
- **Environment variables lich sets in every session:** `LICH_PORT`, `LICH_TOKEN`, `LICH_SESSION_ID`, `LICH_BIN`,
  `LICH_PROJECT_DIR` and `LICH_WORKTREE_PORT`.
- **Environment variables lich reads at startup:** `LICH_SHELL`, `LICH_LISTEN_PORT` and `LICH_LOG_LEVEL`.

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
