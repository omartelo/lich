# Contract: session title

Reports Claude Code's auto-generated session title (the `ai-title`) so lich can
name the session card after it — the same short summary shown in
`claude --resume`, instead of `Session 3`.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-title?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "title": "<ai-title>"}
```

- `session_id` — the lich card, from `LICH_SESSION_ID`.
- `title` — the latest `ai-title` for the session. lich trims it and rejects an
  empty result.

Responses: `204` ok · `401` invalid token · `400` invalid body · `500` lich
failed to persist.

## Event → action mapping

| Claude Code hook | action                                              |
|------------------|-----------------------------------------------------|
| `Stop`           | set the session label to `title` (if still auto)    |

The `ai-title` is an internal Haiku summary of the first prompt, written to the
transcript **after** the first turn — so it does not exist at `SessionStart`.
The `Stop` hook is the earliest reliable point: read the transcript path Claude
Code passes on stdin and take the last `ai-title` line:

```sh
title=$(tac "$transcript_path" | grep -m1 '"type":"ai-title"' | jq -r '.aiTitle')
```

Send it on `Stop`. Re-sending on every `Stop` is fine — lich only applies it
while the label is still automatic (see below), so a stable title is idempotent.

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionTitle`:
  validates the token and body (`parseSessionTitle`), then forwards
  `(session_id, title)`.
- **Guarded write** — `internal/store/mutations.go`, `Service.SetSessionTitle`:
  `UPDATE sessions SET label = ? WHERE id = ? AND label_auto = 1`. A user
  `RenameSession` clears `label_auto`, so a manual name is never stomped.
  Returns whether the label actually changed.
- **Live update** — `internal/terminal/terminal.go`: when the label changed,
  emits the global Wails event `session-title` (`{id, label}`);
  `frontend/src/lib/projects.tsx` mirrors it into session state so the card
  updates without a reload.

## Known ceilings

- **Only overwrites an automatic label.** Once the user renames a session, the
  title stops applying to it — by design (option A). There is no "revert to
  auto" today; renaming to the exact default does not re-arm it.
- **`ai-title` is internal and undocumented.** Format (`{"type":"ai-title",
  "aiTitle":...}` in the transcript jsonl) can change between Claude Code
  versions. The hook must swallow extraction failures and no-op — session state
  and the rest of lich keep working if it breaks.
- **Reacts to the first prompt, not later pivots reliably.** Claude Code may or
  may not refresh the `ai-title` mid-session; lich applies whatever the hook
  last sent while the label is still auto.
