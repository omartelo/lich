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

| Claude Code hook          | Codex hook                | Antigravity hook          | opencode event    | oh-my-pi event                | Crush hook | Cursor CLI hook | Kiro CLI hook | action                                           |
|---------------------------|---------------------------|---------------------------|-------------------|-------------------------------|------------|-----------------|---------------|--------------------------------------------------|
| `PostToolUse` + `Stop`    | `PostToolUse` + `Stop`    | `PreInvocation` + `Stop`  | `session.updated` | `session_stop` + `turn_start` | —          | —               | —             | set the session label to `title` (if still auto) |

**Kiro registers no title report**, and the reason is not that it has no title:
it writes one into its session metadata (`title`, derived from the first
prompt). The report is a *script that reads a transcript path off the hook's own
stdin*, and Kiro passes no path on any of its five events — its payloads carry
`cwd`, `session_id` and the event's own fields and nothing else (measured on
2.21.0). Closing it would mean lich resolving that file itself, which is a
different mechanism from this contract rather than another column in it.

The `ai-title` is an internal Haiku summary of the first prompt. It does not
exist at `SessionStart`, but it lands long before the turn it belongs to ends:
Claude Code fires the title call **in parallel with the turn's own first model
call**, and the title reaches the transcript about three seconds after the
prompt. Read the transcript path the harness passes on stdin and take the last
`ai-title` line:

```sh
title=$(grep '"type":"ai-title"' "$transcript_path" | tail -n1 | jq -r '.aiTitle')
```

**Which is why the turn's end is the wrong place to read it, and the only place
lich used to.** A first turn that runs for ten minutes showed `Session 3` for
all ten, with its real name sitting on disk since second three, and a turn the
user interrupted never got a name at all. So the report is sent from the first
event of the turn that can carry it — `PostToolUse`, or `PreInvocation` where a
harness has no post-tool event of its own — and again at `Stop`, which stays the
backstop for a turn that ran no tools and the one path that still sees a later
retitle.

**The in-turn report latches; the `Stop` one does not.** A title is found by
scanning a transcript that grows all turn — 324 ms on a 168 MB one, measured on
a warm cache — and a turn with fifty tool calls would pay that fifty times to
learn what it knew after the first. So a client that reports in-turn records
that it found a title for this session and stops looking, which makes the new
path one scan per session. `Stop` keeps scanning: it is what a retitle arrives
on, and lich's own guard below is what makes re-sending free.

A provider that generates no title sends whatever it names its own thread after
— Codex uses the first user message verbatim, which the plugin reads from the
rollout and trims to 80 characters, and Antigravity is read the same way, from
the `transcriptPath` its payloads carry: the first `USER_INPUT` entry is that
message. The contract only asks for a non-empty string; where it comes from, and
where it is cut, is the client's business.

Those two are also why the in-turn report is worth registering on a harness that
generates nothing: their title is the user's own first message, which exists
before the turn does. Only Claude has to wait the three seconds.

**A client that trims does it by codepoint, never by byte.** lich takes the
string as sent and the card truncates what will not fit, so a title cut through
the middle of a character is not rejected anywhere — it reaches the card as a
trailing `U+FFFD`, and only when a multibyte character straddles byte 80. Which
character that is depends on every multibyte one before it, so the 80th
character being ASCII does not make a title safe: `é` + 77 ASCII + `é` splits on
its second `é` while the 80th character is a plain `b`. `cut -c` is the way to
get there, and how badly depends on whose `cut` it is: GNU coreutils counts
characters where the locale says UTF-8 and bytes under `LC_ALL=C`, while
BusyBox counts bytes either way — measured on coreutils 9.11 and BusyBox 1.38.0
against the string above. So the locale is an escape hatch on a glibc box and
none at all on Alpine, and a client that reaches for `cut -c` cannot know which
it will run under. Cut in something that counts codepoints — `jq`'s `$s[0:80]`
is already in reach of any hook that builds its body with `jq` — rather than
reasoning about the environment.

That environment is otherwise passed straight through: lich hands the PTY what
it was launched with (`internal/terminal/childenv.go` drops AppImage internals
and nothing else, and the sandbox sets only `HOME`), so a session started from
a desktop carries that desktop's locale and one started without one carries
none. The report is the same shape either way — which is what makes it a thing
to write down rather than to catch.

Send it from the turn's first event that can carry it, and again on `Stop`.
Re-sending is fine — lich only applies it while the label is still automatic
(see below), so a stable title is idempotent, and the guard is what lets a
client report early without having to know whether it already did.

opencode is the one harness that hands the title over instead of being read out
of a transcript: every `session.updated` carries the whole session, title
included, and the plugin forwards it when it changed. oh-my-pi hands it over too
— off the session manager every event carries — but writes it asynchronously
after the turn, which is why the client reads it again on the next `turn_start`
and sends only what changed. It arrives more than once
per turn, which the idempotence above already covers. **Crush has no title to
report** — its only event is `PreToolUse`, whose payload is about the tool — so a
Crush card keeps the name lich gave it.

**Cursor now runs this path for the first time, and still reports nothing.** lich
installs no plugin there; the CLI executes the Claude Code registration, and of
the events that registration names it delivers `PostToolUse` but not `Stop`
(`README.md`) — so a Cursor session never reached this report at all while it
was sent at the turn's end, and reaches it now. It gets no further than a guard:
either the payload names no readable transcript, or the transcript it names
carries none of the three shapes read above. Nothing is sent, nothing latches,
and the card keeps the name lich gave it — the same outcome as before, through a
code path that now runs, which is worth knowing when a Cursor card is the one
being debugged.

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
- **A very short first prompt is never titled at all.** Claude Code does not
  fire its title call for a first prompt under 10 characters, so no `ai-title`
  is ever written and no amount of reading finds one — a card whose whole prompt
  was `1+1 is?` keeps the name lich gave it, correctly, since the prompt would
  fit on the card anyway. Absence there is the CLI's behaviour, not a broken
  hook, and it is the first thing to rule out before debugging one.
- **Reacts to the first prompt, not later pivots reliably.** Claude Code may or
  may not refresh the `ai-title` mid-session; lich applies whatever the hook
  last sent while the label is still auto.
