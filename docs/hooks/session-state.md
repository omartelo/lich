# Contract: session state

Reports a session's processing state to lich so its card shows a spinner while
the agent is working, a check when the turn ends, and a bell when it is blocked
on the user — plus a toast that routes to the waiting card. A `busy` report may
also name the tool the agent is about to run, and a `waiting` what it is blocked
on; the card shows either one under the session's label.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/hook?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>", "state": "<busy|done|waiting|idle>",
 "tool": "<tool name>", "detail": "<what it acts on>",
 "reason": "<what the agent is blocked on>"}
```

States: `busy`, `done`, `waiting`, `idle`. lich rejects anything else — the
`interrupted` state it publishes to its own window included (see below).

`tool` and `detail` are optional and belong to the pre-tool report alone (see
below). Both are trimmed and capped at 120 characters — they decorate a state
that has to land either way, so an over-long value is truncated, never a reason
to reject the report. A `detail` with no `tool` to qualify names nothing and is
dropped.

`reason` is the same kind of field for the other half of the contract: optional,
trimmed, capped at 120, and belonging to the `waiting` report alone — it says
what the agent is blocked on, and no other state is a question. A `reason` on
any other state is dropped, and one that is absent, empty or over-long is never
a reason to refuse the report: the bell has to land either way.

Responses: `204` ok · `401` invalid token · `400` invalid body.

Both sides test against the payloads in
[`fixtures/session-state.jsonl`](fixtures/session-state.jsonl).

## Event → state mapping

| Claude Code hook   | Codex hook          | Antigravity hook | opencode event           | oh-my-pi event | Crush hook | Cursor CLI hook    | Kiro CLI hook      | state     |
|--------------------|---------------------|------------------|--------------------------|----------------|------------|--------------------|--------------------|-----------|
| `UserPromptSubmit` | `UserPromptSubmit`  | `PreInvocation`  | `session.status` (`busy`) | `input`        | —          | —                  | `userPromptSubmit` | `busy`    |
| `PreToolUse`       | `PreToolUse`        | `PreToolUse`     | `tool.execute.before`    | `tool_call`    | —          | dropped            | `preToolUse`       | `busy` + `tool` |
| `PostToolUse`      | `PostToolUse`       | —                | `tool.execute.after`     | `turn_start`   | —          | dropped            | `postToolUse`      | `busy`    |
| `Notification`     | `PermissionRequest` | —                | any `*.asked`            | —              | —          | —                  | —                  | `waiting` + `reason` |
| `Stop`             | `Stop`              | `Stop`           | `session.status` (`idle`) | `session_stop` | —          | —                  | `stop`             | `done`    |
| `SessionEnd`       | —                   | —                | —                        | —              | —          | `SessionEnd`       | —                  | `idle`    |

**Kiro closes four of the six rows and neither of the other two.** It has no
permission event, so a Kiro session waiting on a confirmation reads as `busy`
rather than `waiting` — the card says it is working, which is true, and does not
say what it is waiting for. It has no session-end event either, so the card keeps
the provider's mark until the PTY itself goes (docs/ceilings.md).

Kiro is also the one harness that reads a hook's **stdout back into the
conversation as context** — that is what its hooks are for. Every report script
is silent by contract already, which is what makes them observations; on Kiro
that stops being a style choice, because anything printed would arrive in front
of the model as if the user had typed it.

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

**Antigravity says `busy` before the model rather than after a tool.** It has no
prompt event: `PreInvocation` fires before every model call, which is the turn's
own first step, so it carries both the session id and `busy`. Its `PostToolUse`
is registered for the touched contract only — `PreInvocation` runs again between
tool calls, so a `busy` there would say what has just been said. Nothing reports
`waiting`: Antigravity raises no event when it asks for permission that has been
measured, and an unmeasured event name is a report that silently never fires, so
an Antigravity card shows the spinner while the agent waits on you. Nothing
reports `idle` either — there is no event for the CLI leaving.

**Antigravity reads a verdict off every hook's stdout**, which is why its
registration appends one to each command rather than letting the shared scripts
answer. Measured on 1.1.19: an empty stdout leaves the tool call to Antigravity's
own permission check, `{"decision":"allow"}` does the same — the hook declines to
block, and a command the user has not permitted is still refused — and `{}`, an
object carrying no `decision` at all, **denies** it. The trap is that last one: a
report that starts printing JSON without a verdict stops that session from using
tools, and nothing says why.

**Crush reports no state at all.** Its only hook event is `PreToolUse`, and a
`busy` with nothing that can end it would leave a spinner on the card until the
next turn — a state that is wrong for longer than it is right. So the plugin
registers this contract on the three harnesses that can close it, and a Crush
card carries no indicator. The day Crush ships `Stop` (its `docs/hooks/FUTURE.md`
tracks the request, not the event), the column fills in from the existing script.

`Notification` fires when Claude needs a permission decision **or** when the
session has simply been sitting at its prompt with nothing to do. Only the first
of those blocks a human, and lich reads the difference off the report before it
rather than off the event, so the client reports `waiting` for both and **lich
decides which one arrived** (see `turnLog` below). Claude Code does now label the
two — `notification_type` is one of 14 values, `permission_prompt` and
`idle_prompt` among them (2.1.240) — but that is one harness of five: Codex's
`PermissionRequest` and opencode's `.asked` events carry no such label, and a
plugin older than the field sends nothing. The rule has to hold for all of them,
so the label is not what decides. Codex has no `Notification`:
the permission half arrives as `PermissionRequest`, where a hook exiting `2`
would *deny* the request — which is why the contract's client rules (silent,
always exit 0) are load-bearing there and not merely polite.

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
their documentation. MCP tools are absent from the table above because no two
harnesses name them alike; they have their own table below. A harness is free to
report a name outside either table; the card shows whatever arrives, minus the
MCP machinery it can prove — that prefix costs a card its whole width before the
part worth reading starts:

| Harness           | MCP tool name             | On the card                       |
|-------------------|---------------------------|-----------------------------------|
| Claude Code       | `mcp__lich__open_session` | `lich · open_session`             |
| Codex             | `mcp__srv__tool`          | `srv · tool`                      |
| Antigravity       | `call_mcp_tool`           | `call_mcp_tool · lich/open_session` |
| oh-my-pi          | `mcp__lich_list_sessions` | `lich_list_sessions`              |
| opencode          | `lichprobe_list_sessions` | unchanged                         |

Measured against opencode 1.18.18, omp 17.3.7 and Antigravity 1.1.19 by running
each CLI against a `lich mcp` server and reading the name off the handler the
plugin already has — `input.tool`, `event.toolName`, and `toolCall.name` on the
hook payload.

**Antigravity is the one harness whose tool name is not the tool.** Every MCP
call there is the single step `call_mcp_tool`, and which server and which tool
are two of its arguments (`args.ServerName`, `args.ToolName`) — so the client
reads them and sends them as `detail`, which is why that row is the only one
whose card line is made of both fields.

Of the names that *are* the tool, only the doubled underscore can be split:
omp's single one divides `mcp__lich_list_sessions` into `lich` + `list_sessions`
or `lich_list` + `sessions` with nothing in the string to say which, so only its
prefix comes off, and opencode's form carries no marker at all. Crush is absent
because it reports no tool (see above).

`detail` is whatever identifies the call at a glance — the command line, the
file path, the pattern, and on Antigravity the MCP tool its step name does not
carry. It is free text: a harness that offers nothing usable sends only `tool`,
and the card shows only the name. The client is what makes it readable (the
harnesses report absolute paths, which no 240px card can show); lich takes what
it is given.

**A report holds the tool until the state leaves `busy`.** `PostToolUse` fires
between tools with no `tool` field, so treating "no tool" as "clear" would blink
the line off and on at every step. lich therefore keeps the last name for as
long as the session is `busy`, and drops it on `done`, `waiting` and `idle` —
never on `idle` alone, which Codex has no event for.

`PreToolUse` is the first contract event on the agent's critical path: in both
harnesses a hook exiting `2` there **blocks the tool call**. Until now the worst
a broken script could do was lose a status report.

## What a turn is waiting for

A `waiting` may carry `reason`: one short line naming what the agent is blocked
on, shown on the card in place of its "Waiting on you" and under the session's
name in the toast. Free text, passed through as sent — the same rule the tool
pair follows, and for the same reason: the word on the card should be the word
in the terminal beside it.

The card is where the user decides which of five sessions to open, and a bare
bell makes every one of them look alike: a permission prompt for a destructive
command and a question about which file to touch are the same badge until one is
opened. What each harness can say about it differs, because none of these events
was written to be read this way:

| Harness     | Waiting event          | What the event carries                            | Reason to send            |
|-------------|------------------------|---------------------------------------------------|---------------------------|
| Claude Code | `Notification`         | `message`, `title`, `notification_type`            | `message` — its own sentence, already written for a human |
| Codex       | `PermissionRequest`    | `tool_name`, `tool_input`, and no message of its own | `tool_name`, qualified from `tool_input` the way `detail` already is |
| opencode    | `permission.asked`     | `permission`, `patterns`, `metadata`               | `permission`              |
| opencode    | `permission.v2.asked`  | `action`, `resources`                              | `action`                  |
| opencode    | `question(.v2).asked`  | `questions[]` of `question` / `header` / `options` | `questions[0].header` — the ≤30-char label the question already carries for narrow surfaces |
| oh-my-pi    | —                      | —                                                  | — (reports no `waiting` at all) |
| Crush       | —                      | —                                                  | — (reports no state at all) |

Read off what each harness ships rather than off its documentation: Claude Code
2.1.240's own hook-input builder, the `permission-request.command.input` schema
embedded in Codex 0.147.0, and opencode 1.18.x's `/doc` — which is also where the
four `.asked` types the client matches on are enumerated (`permission.asked`,
`permission.v2.asked`, `question.asked`, `question.v2.asked`).

Only Claude Code hands over a sentence. Codex and opencode name the thing being
asked about and nothing else, so their cards read `Bash` or `edit` where a Claude
card reads "Claude needs your permission to use Bash" — coarser, and still the
difference between opening the right card and opening three. A harness with
nothing to say sends no `reason`, and its card keeps the bare "Waiting on you": a
missing reason never costs a bell.

## lich server side

- **Env injection** — `internal/terminal/terminal.go`, `Service.sessionEnv`:
  adds the three `LICH_*` vars to each PTY's environment.
- **Endpoint** — `internal/terminal/transport.go`, `transport.hook`: validates
  the token and body (`parseHookRequest`) on the same loopback listener as
  terminal I/O, then forwards the whole report. The free text is trimmed and
  capped there, and each field is dropped on the states it does not belong to:
  `detail` with no `tool`, `reason` on anything but `waiting`.
- **UI push** — `internal/terminal/terminal.go`: emits the global app event
  `session-status` (`{id, state, tool, detail, reason}`). Global rather than
  per-session because its consumers outlive any one card.
- **What `waiting` meant** — `internal/terminal/hookstate.go`, `turnLog`: the one
  report lich does not pass through as sent. A permission decision only ever
  happens inside a turn, so the report before it settles which `Notification`
  arrived: after `busy` a human is blocking an open turn and the report is
  published; after `done`, after `idle`, or after nothing at all, the session is
  merely idle at its prompt and **nothing is emitted** — no bell, no toast, no
  desktop notification. The card keeps whatever it was already showing (the
  finished turn, an inbox count, nothing), which is what an unchanged session
  should show. `waiting` itself is not recorded: it interrupts a turn rather than
  replacing it, so two permission prompts in one turn are two blocks.
- **The interrupt lich reads itself** — `internal/terminal/draft.go` and
  `Service.noteInterrupt`: no harness event says "the user stopped this turn"
  (see the ceiling above), so lich takes it from the keystrokes going into the
  PTY, which is the one place every provider is alike. A lone `Ctrl+C` or
  `Escape` — never a byte inside an escape sequence, never one carried in by a
  bracketed paste — ends a turn `turnLog` already has open, and lich emits
  `session-status` with the state **`interrupted`**. That state is outbound
  only: the hook endpoint rejects it like any other unknown word, because it
  says something only lich is in a position to know. It is deliberately not
  `done` — stopping a turn is not finishing one, so an interrupted card must
  not wear the check a completed turn earns. Consumers that know only the four
  contract states read it as "no state" and clear the indicator, which is the
  right reading for a session sitting at its prompt with nothing to show. And
  it is a fallback, never a source of truth: it only ever ends a turn lich
  heard start, so a `Ctrl+C` clearing a line at an idle prompt says nothing, and
  the provider's next report overwrites whatever it concluded.
- **The peer roster** — `internal/relay`, `Observe` and `roster.go`: reads the
  same stream raw, for two questions of its own. Which turn an errand belongs to
  is one (`s.state`, where a mid-turn `waiting` has to keep reading as `busy`).
  What each session is doing, published to an agent listing its peers
  (`Peer.State`, `lich list`, `list_sessions`), is the other — and it applies the
  same rule as `turnLog`: a `waiting` inside a turn is published, because it is
  the state that means "do not send work here", and one outside a turn is not,
  because a session idle at its prompt is the most available it will ever be.
- **The turn's window** — `internal/terminal/turnsnap.go`: reads the same stream
  for a third question, what the turn changed on disk. `busy` opens a window and
  `done` closes it, each taking a `git write-tree` snapshot of the session's
  checkout; the Review panel diffs the pair. Only the first `busy` of a run opens
  anything — the repeat every provider sends between tools would otherwise walk
  the opening snapshot forward through the turn it is meant to precede — and
  `idle` abandons an open window rather than closing it, there being no closing
  report coming. lich's own `interrupted` closes one: a stopped turn is a turn
  that ended, and it changed files like any other. A provider that reports no
  state has no window here at all, which today is Crush and Cursor CLI.
- **Store** — `frontend/src/lib/session/session-status-store.ts`: one subscription taken
  at page load keeps the last state of every session, keyed by id. The card
  cannot hold it: the sidebar only renders cards for the active project, so
  switching projects unmounts them, and a status reported meanwhile would be lost.
  `session-tool-store.ts` reads the same event for the tool pair, keeping its own
  keyed entry so a repeat `busy` — which the status store collapses into no
  change at all — still moves the tool line. The `waiting` reason is held by the
  status store itself, beside the state it belongs to: it is the only field whose
  own state is the thing that clears it, and a second prompt in one turn moves the
  line because the store weighs the reason alongside the state. The status store
  also keeps whether each session's state has been **read**: `markSeen` is called
  for the one session whose terminal is on screen, while the window has focus, and
  a fresh report clears the mark again. Only `done` reads it — the live states say
  what they say whether or not anybody is watching.
- **Render** — `frontend/src/components/sidebar/SessionCard.tsx`: reads the stores
  (`useSessionStatus`, `useSessionTool`, `useSessionWaitingReason`) and shows a
  spinner (`busy`), check (`done`) or bell (`waiting`); any other value, including
  `idle` and `interrupted`, clears the indicator. A `done` is drawn at two weights
  (`SessionStatusIcon`, `useSessionUnread`): solid while the finished turn is
  still unread, faded once the user has watched that card. It is the same mark
  the tab badge and the notification queue read, so one session cannot be news
  in one place and read in another. A reported reason takes the waiting line's
  whole width — the amber glyph beside it is what still says "waiting on you", so
  spending the line on that phrase again would cost the card the only words on it
  the user does not already know. The tool line sits under the session's label and exists only while
  one is reported, so a card outside a turn is exactly the card it was before.
- **Tab badge** — `frontend/src/components/tabs/ProjectTab.tsx`: reduces a
  project's sessions to one indicator (`useProjectStatus`, ranking `waiting` over
  `busy` over `done`), shown only while the project is not the active one. A
  `done` stops badging once *that session's* card has been watched, so a project
  left with three finished agents in it still badges for the two nobody opened;
  `busy` and `waiting` badge for as long as they hold, being live states rather
  than notifications.
- **Toast + route** — `frontend/src/providers/projects.tsx`: raises an actionable toast
  that navigates to the session's card when a report says `waiting`, carrying the
  reason under the session's name, skipped for the session already focused. It reads the raw event rather than the store: the
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

- **An interrupted turn is lich's own reading, not a report.** Three harnesses
  raise nothing when the user stops a turn — measured by driving each CLI at a
  real PTY against a stub listener and pressing the key mid-turn: Claude Code
  and Codex both go quiet after their last `busy` (Codex's rollout keeps the
  `task_started` with no completion beside it), and omp cancels the running
  tool and reports `busy` again without ever raising `session_stop`. opencode is
  the exception: an abort raises `session.status idle` within the same second,
  so its card closes the turn on its own — as a *finished* one, which is the
  only word its event has. lich therefore reads the interrupt off the PTY
  instead (below), and the state it publishes is not `done`.
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
- **A block lich never heard start is not read as a block.** `turnLog` lives in
  memory, so a `waiting` arriving with no `busy` on record for that session — the
  first report after lich restarts under a session already mid-turn — is read as
  an idle prompt and shows nothing. Every provider opens a turn before it can ask
  for a permission, so the normal path never hits this; the card catches up on
  the session's next report either way.
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
- **Only Claude Code says what it is waiting for in words.** Codex's
  `PermissionRequest` and opencode's `.asked` events carry the thing being asked
  about (`tool_name`, `permission`, `action`) and no sentence, so those cards read
  a bare `Bash` or `edit` where a Claude card reads why. oh-my-pi and Crush send
  no `reason` at all, because neither reports `waiting` in the first place (see
  the two bullets above), so their cards keep today's "Waiting on you".
- **A reason is only as fresh as the report that carried it.** It lives beside the
  state in the page's status store, so it clears when the state leaves `waiting`
  and is gone after a reload like the state itself — a session already blocked
  when the page loads shows nothing until its next report. Two prompts in one turn
  do move the line: the store weighs the reason alongside the state, so a repeat
  `waiting` with different words is a change rather than a no-op.
- **The tool line names the call, not its progress.** It appears when the tool
  starts and holds that name for however long the tool runs — a 3-minute build
  and a 30ms read look identical. Nothing reports a tool finishing on its own:
  `PostToolUse` says `busy` with no tool, which by the rule above changes
  nothing.
- Adding another state beyond `busy`/`done`/`waiting`/`idle` is a contract
  change — see the versioning note in the README.
