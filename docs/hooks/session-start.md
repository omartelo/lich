# Contract: session start

Reports the provider conversation id running inside a lich session's PTY, so
lich can persist the link between its own session (the card) and the provider's
session. That id is what lets a restored card offer to resume the conversation
it ran before the last restart, and the key for later features that need to
reach a session's transcript. Claude Code and Codex report one today.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-start?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "provider_session_id": "<provider session id>",
 "provider": "<provider id>"}
```

- `session_id` — the lich card, from `LICH_SESSION_ID`.
- `provider_session_id` — the provider CLI's own session id; for both Claude
  Code and Codex, the hook payload's `session_id` field on stdin. Must be
  non-empty.
- `provider` — which CLI is reporting, as a provider id from
  `internal/providers.Registry` (`claude`, `codex`, `opencode`, `crush`).
  Optional: absent means `claude`, the only provider that reported before the
  field existed. An id outside the registry is rejected, like an unknown state
  on `/hook` — lich ships its side of a contract first, so a provider it has no
  entry for is a client running ahead of it.
- `claude_session_id` — **deprecated** alias for `provider_session_id`, still
  accepted so plugin releases before v0.3.0 keep working. When both are
  present, `provider_session_id` wins. New clients must not send it.

Responses: `204` ok · `401` invalid token · `400` invalid body · `500` lich
failed to persist.

## Event → action mapping

| Claude Code hook | Codex hook     | action                                       |
|------------------|----------------|----------------------------------------------|
| `SessionStart`   | `SessionStart` | store `provider_session_id` on the lich       |
|                  |                | session row, and mark the card as running     |
|                  |                | `provider` (the `session-agent` app event)    |

`SessionStart` fires on startup, resume, `/clear` and compaction. A resume
reports the resumed session's id and overwrites the stored value — lich always
holds the id of the provider session currently in the card.

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionStart`:
  validates the token and body (`parseSessionStart`) on the same loopback
  listener as terminal I/O, folds the deprecated `claude_session_id` into
  `provider_session_id`, defaults an absent `provider` to `claude` and rejects
  an unregistered one, then forwards `(session_id, provider_session_id,
  provider)`.
- **Persistence** — `internal/store/mutations.go`, `Service.SetProviderSession`:
  `UPDATE sessions SET provider_session_id`. Surfaced on `store.Session`
  (`providerSessionId`) and returned by `LoadState`.
- **UI push** — after persisting, the same closure (`internal/terminal/terminal.go`,
  `New`) emits the global app event `session-agent` (`{id, agent: <provider>}`):
  a report is proof that provider's CLI runs in this PTY, so a shell card wears
  its icon while it does. The mark lives in
  `frontend/src/lib/session/session-agent-store.ts`, never the store: it clears on the
  session-state contract's `idle` (SessionEnd — the CLI left) and on every PTY
  spawn (the backend emits an empty agent), so it dies with the process that
  earned it. The card's persisted kind — what a respawn runs, what the resume
  prompt keys on — never changes.
- **Consumer** — the resume prompt. `LoadState` hydrates the id onto the
  frontend session (`resumableSession` in `frontend/src/lib/session/sessions.ts`), and
  the first time a restored card is opened `TerminalHost` asks before spawning:
  accepting passes the id to `terminal.Start`, which spawns the kind's resume
  invocation (`resumeArgs` in `internal/terminal/command.go`: `claude --resume
  <id>`, `codex resume <id>`).

## Known ceilings

- **Start races persistence.** The hook can fire before lich has inserted the
  session row (`AddSession`). The `UPDATE` then matches nothing and the id is
  dropped — not an error. In practice Claude's boot is slower than the local
  insert, so this is not observed; if it ever bites, retry from the hook or
  re-report on a later event.
- **A card without the plugin never offers a resume.** The id only exists
  because this hook reported it, so the prompt is a plugin-gated feature: the
  session simply starts fresh, as before.
- **Not the transcript path.** The path is reconstructable from the id and cwd;
  storing it too is a contract change — add a field only when a feature needs
  it, per the versioning note in the README.
- **Only Claude Code and Codex resume.** The field and the column are
  provider-agnostic, but each CLI spells resume its own way (`claude --resume
  <id>`, `codex resume <id>`), so both `resumeArgs`
  (`internal/terminal/command.go`) and `resumableSession`
  (`frontend/src/lib/session/sessions.ts`) list the kinds that have one. A provider
  outside that list reporting an id has it stored and ignored until its own
  invocation is wired.
- **Resume availability is asked of each provider's transcript directory.**
  `ResumeAvailable` (`internal/terminal/resume.go`) globs
  `~/.claude/projects/*/<id>.jsonl` for Claude Code and
  `~/.codex/sessions/*/*/*/rollout-*-<id>.jsonl` for Codex
  (`CLAUDE_CONFIG_DIR` / `CODEX_HOME` override the roots). Both layouts are
  internal to the provider: one that moves its files makes every restored card
  start fresh instead of erroring, which is the direction this fails in.
- **The deprecated `claude_session_id` alias stays until the install gate can
  no longer meet a plugin older than v0.3.0.** Dropping it earlier silently
  breaks resume for anyone who has not updated the plugin.
