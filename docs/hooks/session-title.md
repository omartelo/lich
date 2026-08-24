# Contract: session title

Reports the title a provider gives its own session so lich can name the session
card after it, instead of `Session 3` — for Claude Code the auto-generated
`ai-title`, the same short summary shown in `claude --resume`.

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

Both sides test against the payloads in
[`fixtures/session-title.jsonl`](fixtures/session-title.jsonl).

## Event → action mapping

| Claude Code hook | Codex hook | Antigravity hook | opencode event    | oh-my-pi event                | Crush hook | action                                           |
|------------------|------------|------------------|-------------------|-------------------------------|------------|--------------------------------------------------|
| `Stop`           | `Stop`     | `Stop`           | `session.updated` | `session_stop` + `turn_start` | —          | set the session label to `title` (if still auto) |

The `ai-title` is an internal Haiku summary of the first prompt, written to the
transcript **after** the first turn — so it does not exist at `SessionStart`.
The `Stop` hook is the earliest reliable point: read the transcript path Claude
Code passes on stdin and take the last `ai-title` line:

```sh
title=$(tac "$transcript_path" | grep -m1 '"type":"ai-title"' | jq -r '.aiTitle')
```

A provider that generates no title sends whatever it names its own thread after
— Codex uses the first user message verbatim, which the plugin reads from the
rollout and trims to 80 characters, and Antigravity is read the same way: its
`Stop` payload carries a `transcriptPath` whose first `USER_INPUT` entry is that
message. The contract only asks for a non-empty string; where it comes from, and
where it is cut, is the client's business.

**A client that trims does it by codepoint, never by byte.** lich takes the
string as sent and the card truncates what will not fit, so a title cut through
the middle of a character is not rejected anywhere — it reaches the card as a
trailing `U+FFFD`, and only for a title whose 80th character is not ASCII.
`cut -c` is the way to get there: it counts characters only where the locale
says UTF-8, and bytes under `LC_ALL=C`. lich passes the environment it was
launched with straight into the PTY (`internal/terminal/childenv.go` drops
AppImage internals and nothing else, and the sandbox sets only `HOME`), so a
session started from a desktop carries that desktop's locale and never sees
this. One started where the locale is not set does, and the report is the same
shape either way — which is what makes it a thing to write down rather than to
catch.

Send it on `Stop`. Re-sending on every `Stop` is fine — lich only applies it
while the label is still automatic (see below), so a stable title is idempotent.

opencode is the one harness that hands the title over instead of being read out
of a transcript: every `session.updated` carries the whole session, title
included, and the plugin forwards it when it changed. oh-my-pi hands it over too
— off the session manager every event carries — but writes it asynchronously
after the turn, which is why the client reads it again on the next `turn_start`
and sends only what changed. It arrives more than once
per turn, which the idempotence above already covers. **Crush has no title to
report** — its only event is `PreToolUse`, whose payload is about the tool — so a
Crush card keeps the name lich gave it.

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionTitle`:
  validates the token and body (`parseSessionTitle`), then forwards
  `(session_id, title)`.
- **Guarded write** — `internal/store/mutations.go`, `Service.SetSessionTitle`:
  `UPDATE sessions SET label = ? WHERE id = ? AND label_auto = 1`. A user
  `RenameSession` clears `label_auto`, so a manual name is never stomped.
  Returns whether the label actually changed.
- **Live update** — `internal/terminal/terminal.go`: when the label changed,
  emits the global app event `session-title` (`{id, label}`);
  `frontend/src/providers/projects.tsx` mirrors it into session state so the card
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
