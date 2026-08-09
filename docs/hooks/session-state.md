# Contract: session state

Reports a session's Claude Code processing state to lich so its card shows a
spinner while Claude is working, a check when the turn ends, and a bell when
Claude is blocked on the user — plus a toast that routes to the waiting card.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/hook?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "state": "<busy|done|waiting|idle>"}
```

States: `busy`, `done`, `waiting`, `idle`. lich rejects anything else.

Responses: `204` ok · `401` invalid token · `400` invalid body.

## Event → state mapping

| Claude Code hook   | Codex hook          | state     |
|--------------------|---------------------|-----------|
| `UserPromptSubmit` | `UserPromptSubmit`  | `busy`    |
| `PostToolUse`      | `PostToolUse`       | `busy`    |
| `Notification`     | `PermissionRequest` | `waiting` |
| `Stop`             | `Stop`              | `done`    |
| `SessionEnd`       | —                   | `idle`    |

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

## lich server side

- **Env injection** — `internal/terminal/terminal.go`, `Service.sessionEnv`:
  adds the three `LICH_*` vars to each PTY's environment.
- **Endpoint** — `internal/terminal/transport.go`, `transport.hook`: validates
  the token and body (`parseHookRequest`) on the same loopback listener as
  terminal I/O, then forwards `(session_id, state)`.
- **UI push** — `internal/terminal/terminal.go`: emits the global app event
  `session-status` (`{id, state}`). Global rather than per-session because its
  consumers outlive any one card.
- **Store** — `frontend/src/lib/session/session-status-store.ts`: one subscription taken
  at page load keeps the last state of every session, keyed by id. The card
  cannot hold it: the sidebar only renders cards for the active project, so
  switching projects unmounts them, and a status reported meanwhile would be lost.
- **Render** — `frontend/src/components/sidebar/SessionCard.tsx`: reads the store
  (`useSessionStatus`) and shows a spinner (`busy`), check (`done`) or bell
  (`waiting`); any other value, including `idle`, clears the indicator.
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
- The desktop notification is not clickable: clicking it focuses nothing and
  routes nowhere, which is why its text names the session and the project — the
  user navigates by hand. Making it actionable is per-OS and none of it is
  cheap: Linux needs `notify-send --wait --action`, holding a process per
  notification; macOS needs lich shipped as a signed `.app` bundle with
  `UNUserNotificationCenter`; Windows needs a registered AppUserModelID with a
  COM activation handler.
- Adding another state beyond `busy`/`done`/`waiting`/`idle` is a contract
  change — see the versioning note in the README.
