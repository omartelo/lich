# Hook contracts

lich observes and drives Claude Code sessions through **hooks**. Each hook is a
small script that runs inside a session (shipped by the companion plugin
[`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin)) and talks to
lich over a shared local transport.

This directory is the **canonical, contract-first source** for those hooks:

- lich owns the **server side** of every contract — the transport, the
  endpoint, and how the reported data reaches the UI. That side lives in this
  repo and is documented here.
- The plugin owns the **client side** — the hook scripts. The plugin does not
  redefine the protocol; it references the contract documented here and
  implements against it.

Define the contract here first, then implement both sides against it.

## Shared transport

Every hook rides the same loopback channel lich already runs for terminal I/O
(`internal/terminal/transport.go`). lich injects three variables into the
environment of **every PTY it spawns**, inherited by `claude` and its hooks:

| Var               | Purpose                     |
|-------------------|-----------------------------|
| `LICH_PORT`       | endpoint port (loopback)    |
| `LICH_TOKEN`      | auth token (`?token=`)      |
| `LICH_SESSION_ID` | target session/card id      |

Outside lich these are absent, so every hook must no-op and exit 0 — the plugin
stays safe to install globally.

## Client rules (all hooks)

- Missing env vars → no-op, exit 0.
- Short timeout, errors swallowed, always exit 0. A hook must never block or
  fail the user's turn.

## Versioning

- A change **within** an existing contract (a script tweak) is a plugin-only
  release — no lich release needed.
- A change **to** a contract (new endpoint, field, or accepted value) is a
  breaking change: ship the lich server side first, then the plugin. Keep the
  two in lockstep.

## Adding a new hook

1. Write its contract in this directory (transport is already shared; document
   the endpoint, payload, accepted values, and event→action mapping).
2. Implement the lich server side (endpoint handler + however the data reaches
   the UI) with tests.
3. In the plugin, add the hook script and point its doc at the contract here —
   the contract is the single source of truth.

## Contracts

- [session-state.md](session-state.md) — a session's processing state
  (`busy`/`done`) shown on its card.
- [session-start.md](session-start.md) — the Claude session id, persisted
  against the lich session for later features.
- [session-title.md](session-title.md) — Claude's auto-generated `ai-title`,
  applied as the session card's label.
- [session-touched.md](session-touched.md) — a session changed files, so its
  git status refreshes immediately instead of on the next poll.
