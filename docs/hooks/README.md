# Hook contracts

lich observes and drives provider sessions through **hooks**. Each hook runs
inside a session (shipped by the companion plugin
[`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin), which installs
on Claude Code, Codex, Antigravity, opencode, oh-my-pi and Crush) and talks to
lich over a shared local transport. On Claude Code, Codex, Antigravity and Crush
it is a small script the harness runs; opencode and oh-my-pi load a JavaScript
module instead, which is a difference in packaging, not in what a report is.

The contracts are provider-agnostic: lich injects the same variables into every
PTY it spawns, so what changes per provider is only which of its lifecycle
events maps onto a report — each contract's mapping table has a column per
harness. **Cursor CLI's column is the one nobody installs**: lich installs no
plugin there, and the CLI executes every Claude Code hook on the machine — the
user's own and each installed plugin's — so those columns fire from the Claude
Code install or not at all, and a machine without that install has a Cursor
session that reports nothing. Of the nine that install registers, Cursor delivers
`SessionStart`, `PreToolUse`, `PostToolUse` and `SessionEnd` and no others
(measured 2026.08.11, against hooks in its own format and in Claude Code's
alike). "dropped" in the state table is lich refusing a report it would have no
way to end: without `Stop` a `busy` never becomes `done`, so
`terminal.closableState` keeps everything but `idle` off a Cursor card. Reports
also arrive naming `claude`, the argument Claude Code's own registration passes;
lich answers from the kind it spawned instead (`terminal.providerKind`).
`../ceilings.md` carries what all of that costs.

This directory is the **canonical, contract-first source** for those hooks:

- lich owns the **server side** of every contract — the transport, the
  endpoint, and how the reported data reaches the UI. That side lives in this
  repo and is documented here.
- The plugin owns the **client side** — the hook scripts. The plugin does not
  redefine the protocol; it references the contract documented here and
  implements against it.

Define the contract here first, then implement both sides against it.

Prose alone could not hold that line across two repositories — a renamed field
went red in neither. [`fixtures/`](fixtures/) is every contract as bytes, one
case per line, and both sides assert against the same lines: lich in
`internal/terminal/fixtures_test.go`, the plugin in its own suite. A payload
that moves now fails on whichever side moved first.

## Shared transport

Every hook rides the same loopback channel lich already runs for terminal I/O
(`internal/terminal/transport.go`). lich injects three variables into the
environment of **every PTY it spawns**, inherited by the provider CLI and its
hooks:

| Var               | Purpose                     |
|-------------------|-----------------------------|
| `LICH_PORT`       | endpoint port (loopback)    |
| `LICH_TOKEN`      | auth token (`?token=`)      |
| `LICH_SESSION_ID` | target session/card id      |

Outside lich these are absent, so every hook must no-op and exit 0 — the plugin
stays safe to install globally.

These three are the hook contract, not the whole session environment: lich also
exports `LICH_WORKTREE_PORT`, a dev-server port belonging to the session's
checkout, and `LICH_PROJECT_DIR`, the project's own directory (the main
checkout, which for a session running in a worktree is somewhere else entirely).
Neither addresses anything in lich and no hook reads them — they exist for the
project's own setup and run scripts and for commands typed in a card
(`PORT=$LICH_WORKTREE_PORT pnpm dev`,
`cp --reflink=auto -r "$LICH_PROJECT_DIR/node_modules" .`).

`LICH_BIN` is the fourth: the path of the lich this session belongs to, which
the agent in the PTY calls to reach the sessions beside it. That surface has its
own contract in [cli.md](../cli.md).

## Client rules (all hooks)

- Missing env vars → no-op, exit 0.
- Short timeout, errors swallowed, always exit 0. A hook must never block or
  fail the user's turn.

## Versioning

- A change **within** an existing contract (a script tweak) is a plugin-only
  release — no lich release needed.
- A change **to** a contract (new endpoint, field, or accepted value) is a
  breaking change: ship the lich server side first, then the plugin. Keep the
  two in lockstep. The order runs through the fixtures: the prose here moves,
  then [`fixtures/`](fixtures/), then lich's endpoint, then the plugin.

## Adding a new hook

1. Write its contract in this directory (transport is already shared; document
   the endpoint, payload, accepted values, and event→action mapping).
2. Add its [`fixtures/`](fixtures/) file — the payloads it accepts and the ones
   it refuses — and register it in `hookContracts`
   (`internal/terminal/fixtures_test.go`). A contract with no fixture file fails
   that test, so this step is not optional.
3. Implement the lich server side (endpoint handler + however the data reaches
   the UI) with tests.
4. In the plugin, add the hook script and point its doc at the contract here —
   the contract is the single source of truth.

## Contracts

- [session-state.md](session-state.md) — a session's processing state
  (`busy`/`done`/`waiting`/`idle`) shown on its card.
- [session-start.md](session-start.md) — the Claude session id, persisted
  against the lich session for later features.
- [session-title.md](session-title.md) — Claude's auto-generated `ai-title`,
  applied as the session card's label.
- [session-touched.md](session-touched.md) — a session changed files, so its
  git status refreshes immediately instead of on the next poll.

Each has a fixture file of the same name in [`fixtures/`](fixtures/); the format
is documented there.
