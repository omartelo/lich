# Contract: session state

Reports a session's processing state to lich so its card shows a spinner while
the agent is working, a check when the turn ends, and a bell when it is blocked
on the user — plus a toast that routes to the waiting card. A `busy` report may
also name the tool the agent is about to run, which the card shows under the
session's label.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/hook?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "state": "<busy|done|waiting|idle>",
 "tool": "<tool name>", "detail": "<what it acts on>"}
```

States: `busy`, `done`, `waiting`, `idle`. lich rejects anything else.

`tool` and `detail` are optional and belong to the pre-tool report alone (see
below). Both are trimmed and capped at 120 characters — they decorate a state
that has to land either way, so an over-long value is truncated, never a reason
to reject the report. A `detail` with no `tool` to qualify names nothing and is
dropped.

Responses: `204` ok · `401` invalid token · `400` invalid body.

Both sides test against the payloads in
[`fixtures/session-state.jsonl`](fixtures/session-state.jsonl).

## Event → state mapping

| Claude Code hook   | Codex hook          | opencode event           | oh-my-pi event | Crush hook | state     |
|--------------------|---------------------|--------------------------|----------------|------------|-----------|
| `UserPromptSubmit` | `UserPromptSubmit`  | `session.status` (`busy`) | `input`        | —          | `busy`    |
| `PreToolUse`       | `PreToolUse`        | `tool.execute.before`    | `tool_call`    | —          | `busy` + `tool` |
| `PostToolUse`      | `PostToolUse`       | `tool.execute.after`     | `turn_start`   | —          | `busy`    |
| `Notification`     | `PermissionRequest` | any `*.asked`            | —              | —          | `waiting` |
| `Stop`             | `Stop`              | `session.status` (`idle`) | `session_stop` | —          | `done`    |
| `SessionEnd`       | —                   | —                        | —              | —          | `idle`    |

opencode is the one harness that reports a state rather than an event: its
`session.status` carries `busy`, `idle` or `retry` for the session named in the
same payload. Its `idle` means the turn ended, which is lich's `done` — not
lich's `idle`, which says the CLI itself has left. Nothing in opencode's event
list says that, so like Codex it never reports `idle`.

**oh-my-pi carries `busy` on `turn_start` rather than after a tool.** Its
non-interactive runs never emit `input` at all, so the turn boundary is the one
event every turn passes through — the same place the title is re-read, since omp
writes it asynchronously after the turn.

**opencode's `waiting` is a rule, not an event name.** It asks the user in more
than one way — a permission decision and an interactive question, each with a
`.v2.` spelling published alongside — so the client reports `waiting` for any
event type ending in `.asked` rather than for a list of names. Its catalogue is
not exhaustive to enumerate against: a real run emits types (`server.heartbeat`)
that the 94 event schemas its own `/doc` publishes do not list. Nothing is
registered for the answer: replying raises `session.status busy` and dismissing
raises `session.status idle`, each within ~100ms, so the card re-arms itself.

**Crush reports no state at all.** Its only hook event is `PreToolUse`, and a
`busy` with nothing that can end it would leave a spinner on the card until the
next turn — a state that is wrong for longer than it is right. So the plugin
registers this contract on the three harnesses that can close it, and a Crush
card carries no indicator. The day Crush ships `Stop` (its `docs/hooks/FUTURE.md`
tracks the request, not the event), the column fills in from the existing script.

`Notification` fires when Claude needs a permission decision or has been idle
waiting for input — both mean "your turn"; lich shows a toast (see below) only
for `waiting`. Codex has no `Notification`: the same meaning arrives as
`PermissionRequest`, where a hook exiting `2` would *deny* the request — which
is why the contract's client rules (silent, always exit 0) are load-bearing
there and not merely polite.

`SessionEnd → idle` clears the card's indicator (no spinner/check/bell). It
fires when the Claude session ends or is reset, so a stale state does not linger
on a dead session, and a `/clear` starts the next session with a clean card.

`waiting` clears the moment Claude resumes. A typed reply raises
`UserPromptSubmit`, but a **permission approval, plan approval or answered
question does not** — those resume by running a tool, so `PostToolUse → busy` is
what re-arms the spinner after them. Every tool re-reports `busy` (idempotent);
`Stop → done` ends the turn.

## The tool a turn is running

`PreToolUse` reports `busy` like every other event, and adds two fields: `tool`,
the harness's own name for what is about to run, and `detail`, its own words for
what that tool acts on. Both are passed through as sent — lich does not
translate between vocabularies, because the word on the card should be the word
in the terminal beside it.

The vocabularies overlap more than the harnesses' own tool sets do. Codex's
hook payload reports a shell call as `Bash`, the same name and the same
`tool_input.command` string Claude Code sends, and routes reading and searching
through it; the one word of its own that reaches a card is `apply_patch`.
opencode spells the same set in lower case:

| Action        | Claude Code                | Codex                    | opencode         |
|---------------|----------------------------|--------------------------|------------------|
| run a command | `Bash`                     | `Bash`                   | `bash`           |
| edit a file   | `Edit` / `Write`           | `apply_patch`            | `edit` / `write` |
| read a file   | `Read`                     | — (goes through `Bash`)  | `read`           |
| search        | `Grep` / `Glob`            | — (goes through `Bash`)  | `grep` / `glob`  |

Each row was taken off a real run of the CLIs against a stub listener, not from
their documentation — except opencode's MCP tools, which are namespaced by their
server and were not observed here, which is why the row Claude Code and Codex
both spell `mcp__srv__tool` is absent above. A harness is free to report a name
outside this table; the card shows whatever arrives.

`detail` is whatever identifies the call at a glance — the command line, the
file path, the pattern. It is free text: a harness that offers nothing usable
sends only `tool`, and the card shows only the name. The client is what makes
it readable (both harnesses report absolute paths, which no 240px card can
show); lich takes what it is given.

**A report holds the tool until the state leaves `busy`.** `PostToolUse` fires
between tools with no `tool` field, so treating "no tool" as "clear" would blink
the line off and on at every step. lich therefore keeps the last name for as
long as the session is `busy`, and drops it on `done`, `waiting` and `idle` —
never on `idle` alone, which Codex has no event for.

`PreToolUse` is the first contract event on the agent's critical path: in both
harnesses a hook exiting `2` there **blocks the tool call**. Until now the worst
a broken script could do was lose a status report.

## lich server side

- **Env injection** — `internal/terminal/terminal.go`, `Service.sessionEnv`:
  adds the three `LICH_*` vars to each PTY's environment.
- **Endpoint** — `internal/terminal/transport.go`, `transport.hook`: validates
  the token and body (`parseHookRequest`) on the same loopback listener as
  terminal I/O, then forwards the whole report.
- **UI push** — `internal/terminal/terminal.go`: emits the global app event
  `session-status` (`{id, state, tool, detail}`). Global rather than per-session
  because its consumers outlive any one card.
- **Store** — `frontend/src/lib/session/session-status-store.ts`: one subscription taken
  at page load keeps the last state of every session, keyed by id. The card
  cannot hold it: the sidebar only renders cards for the active project, so
  switching projects unmounts them, and a status reported meanwhile would be lost.
  `session-tool-store.ts` reads the same event for the tool pair, keeping its own
  keyed entry so a repeat `busy` — which the status store collapses into no
  change at all — still moves the tool line.
- **Render** — `frontend/src/components/sidebar/SessionCard.tsx`: reads the stores
  (`useSessionStatus`, `useSessionTool`) and shows a spinner (`busy`), check
  (`done`) or bell (`waiting`); any other value, including `idle`, clears the
  indicator. The tool line sits under the session's label and exists only while
  one is reported, so a card outside a turn is exactly the card it was before.
- **Tab badge** — `frontend/src/components/tabs/ProjectTab.tsx`: reduces a
  project's sessions to one indicator (`useProjectStatus`, ranking `waiting` over
  `busy` over `done`), shown only while the project is not the active one. A
  `done` stops badging once the project has been on screen; `busy` and `waiting`
  badge for as long as they hold, being live states rather than notifications.
- **Toast + route** — `frontend/src/providers/projects.tsx`: raises an actionable toast
  that navigates to the session's card when a report says `waiting`, skipped for
  the session already focused. It reads the raw event rather than the store: the
  store collapses a repeat state into no notification, which would swallow a
  toast.
- **Desktop notification** — same handler, `system.Notify`
  (`internal/system/system.go`, zenity's per-OS notifier). It answers to
  `document.hasFocus()`, not to the toast's rule: with the window unfocused every
  `waiting` session is unseen, the focused one included, so that exclusion does
  not apply here. The page decides, the backend only delivers — focus is a fact
  only the page holds. Gated on an opt-in (`lich.notifications.desktop`, tri-state
  so "refused" and "not asked" stay apart): the first report that would have
  notified puts the dialog instead (`NotificationsOptIn`), and **Settings ›
  Notifications** owns the switch from then on.

## Known ceilings

- `UserPromptSubmit` → busy, `Stop` → done. An interrupt (Esc) that skips `Stop`
  can leave a spinner until the next turn resets it.
- Status is retained in the page, not in Go: a reload starts the store empty, so
  a session Claude is already working on shows no indicator until its next
  report. The PTY is backend-owned and survives the reload, so it does keep
  running. Same for the ~1s the `/events` socket takes to reconnect — the hub
  drops what it emits with no client attached. Fix path: keep the last state per
  session in Go and hand it to the store on connect.
- The attention toast auto-dismisses on a timer (`ATTENTION_TOAST_MS`); it does
  not clear when the session leaves `waiting` (user handled it in the terminal).
  Fix path: track the toast id per session and dismiss it on the next
  busy/done.
- **Two `waiting` reports for one prompt are two toasts and two desktop
  notifications.** The toast reads the raw event by design (a repeat state is no
  change to the store, which would swallow the second one), and
  `decideStatusNotice` weighs `previous` for `done` but never for `waiting` — a
  session already waiting notifies again. Harmless while a harness raises one
  event per prompt, which every one of them does today; it is the cost a client
  pays for reporting the same block twice, and the reason to dedupe on the
  client rather than here.
- `PostToolUse → busy` recovers from `waiting` only when Claude runs a tool. Deny
  a permission and let Claude end the turn without another tool and the card
  stays `waiting` until `Stop → done`. Rare, and it self-corrects on the next
  turn.
- **Codex never reports `idle`.** It has no `SessionEnd` event (0.144.5 ships a
  schema for every other event in the table and none for that one), so a Codex
  card keeps its last indicator — and the `session-agent` mark from
  [session-start](session-start.md) — after the CLI exits. Both clear on the
  next PTY spawn, so what lingers is a check on a card whose provider has left.
  The plugin registers a `SessionEnd` hook anyway: an unknown event name is
  ignored rather than rejected, so it starts working the day Codex adds one.
- **opencode never reports `idle` either**, for its own reason: its plugin is
  loaded by the opencode server, so the event that would say "the CLI has left"
  is the server going away with the plugin inside it. Nothing survives to report
  it. An opencode card therefore keeps its last indicator until lich respawns
  that PTY, exactly like a Codex one.
- **oh-my-pi reports neither `waiting` nor `idle`.** Its extension runs
  in-process, so `idle` dies with the process that would send it, exactly as
  opencode's does. `waiting` is absent for a different reason: omp declares a
  `tool_approval_requested` event, but no real run was ever observed emitting it
  — and a report wired to an event name that never fires is one that silently
  never arrives. So an omp session waiting on a permission shows a spinner
  rather than a bell, which Settings says out loud (`OMP_APPROVAL_HINT`) because
  a missing bell reads as a broken install.
- **An opencode report is only as precise as the server it runs in.** The plugin
  reads `LICH_SESSION_ID` from the environment of the process it was loaded by,
  and one opencode server can hold several conversations. Sub-sessions (the
  `task` tool) are excluded by their `parentID` — without that, a sub-agent
  finishing would report `done` while the real turn is still running — but two
  *top-level* sessions in one server would both report onto the same card. A
  session lich spawns gets its own server, so this is the shape of the failure,
  not something the normal path hits.
- The desktop notification is not clickable: clicking it focuses nothing and
  routes nowhere, which is why its text names the session and the project — the
  user navigates by hand. Making it actionable is per-OS and none of it is
  cheap: Linux needs `notify-send --wait --action`, holding a process per
  notification; macOS needs lich shipped as a signed `.app` bundle with
  `UNUserNotificationCenter`; Windows needs a registered AppUserModelID with a
  COM activation handler.
- **The tool line names the call, not its progress.** It appears when the tool
  starts and holds that name for however long the tool runs — a 3-minute build
  and a 30ms read look identical. Nothing reports a tool finishing on its own:
  `PostToolUse` says `busy` with no tool, which by the rule above changes
  nothing.
- Adding another state beyond `busy`/`done`/`waiting`/`idle` is a contract
  change — see the versioning note in the README.
