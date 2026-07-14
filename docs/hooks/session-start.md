# Contract: session start

Reports the Claude Code session id running inside a lich session's PTY, so lich
can persist the link between its own session (the card) and Claude's session.
That id is the key for later features that need to reach a session's transcript
or resume it; nothing in the UI changes today.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-start?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "claude_session_id": "<claude session id>"}
```

- `session_id` — the lich card, from `LICH_SESSION_ID`.
- `claude_session_id` — Claude Code's own session id, from the hook payload's
  `session_id` field on stdin. Must be non-empty.

Responses: `204` ok · `401` invalid token · `400` invalid body · `500` lich
failed to persist.

## Event → action mapping

| Claude Code hook | action                                              |
|------------------|-----------------------------------------------------|
| `SessionStart`   | store `claude_session_id` on the lich session row   |

`SessionStart` fires on startup, resume, `/clear` and compaction. A resume
reports the resumed session's id and overwrites the stored value — lich always
holds the id of the Claude session currently in the card.

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionStart`:
  validates the token and body (`parseSessionStart`) on the same loopback
  listener as terminal I/O, then forwards `(session_id, claude_session_id)`.
- **Persistence** — `internal/store/mutations.go`, `Service.SetClaudeSession`:
  `UPDATE sessions SET claude_session_id`. Surfaced on `store.Session`
  (`claudeSessionId`) and returned by `LoadState`.

## Known ceilings

- **Start races persistence.** The hook can fire before lich has inserted the
  session row (`AddSession`). The `UPDATE` then matches nothing and the id is
  dropped — not an error. In practice Claude's boot is slower than the local
  insert, so this is not observed; if it ever bites, retry from the hook or
  re-report on a later event.
- **`claude_session_id` is stored, not surfaced.** No feature reads it yet; it
  exists so future work (transcript access, resume, the `ai-title` naming hook)
  has the link ready.
- **Not the transcript path.** The path is reconstructable from the id and cwd;
  storing it too is a contract change — add a field only when a feature needs
  it, per the versioning note in the README.
