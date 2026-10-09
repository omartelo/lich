# Contract: mod status

Lets a Claude Code session read the relay errands it is part of, so its status
line can tell the person at the terminal what the card's tooltip and inbox mark
tell them in the window: who this session owes an answer, which of the sessions
it handed work to are still at it, and which answers are waiting to be
collected. The client is the same Claude Code **mod** as
[mod-control.md](mod-control.md) (Claude Code 2.1.280 or later, shipped by the
companion plugin), in a module of its own.

It is the one contract that reads rather than reports: nothing the mod sends
changes anything in lich, and reading it **never collects**. An answer listed
under `ready` stays in the session's inbox until the agent collects it with
`wait_for_answer` or `lich wait`; a status line that drained the inbox would
take the answer away from the agent it was meant for.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
GET http://127.0.0.1:${LICH_PORT}/mod/status?token=${LICH_TOKEN}&session_id=${LICH_SESSION_ID}
X-Lich-Plugin: <plugin release>
```

lich answers at once; it never holds the request.

```
200 OK
Content-Type: application/json

{
  "owed": [
    {"ticket": "b19cb405", "from": "Session 26", "asked": "Sou a sessão Claude do lich-plugin"}
  ],
  "open": [
    {"ticket": "4f0c1a2e", "target": "docs", "state": "busy"},
    {"ticket": "9d3e77b0", "target": "tests", "state": "queued"}
  ],
  "ready": [
    {"ticket": "c7a91e04", "target": "review", "status": "answered"}
  ]
}
```

Responses: `200` the status · `401` invalid token · `400` missing `session_id`
· `405` not a GET.

The three lists are always present and never `null`, `[]` when empty. A
`session_id` lich has no errand for, including one it does not know at all,
reads as three empty lists. Every list is oldest first.

- `owed`: the errands this session was handed and has not answered yet, each
  one a caller blocked on `lich reply` / `reply_to_session` with its `ticket`.
  - `from`: the label of the session that asked, as it was when it asked;
    `""` when the request came from the command line rather than a session.
  - `asked`: the opening of the request, on one line, which is what tells two
    open errands apart. Never the whole prompt.
- `open`: the errands this session handed to other sessions that have not
  ended yet.
  - `target`: the label of the session working it, as it was when it was sent.
  - `state`: `queued` while the task waits for the target to reach a prompt;
    after that, the target's last report as
    [session-state](session-state.md) spells it (`busy`, `waiting`, `done`),
    `interrupted` for a turn lich saw stopped, and `""` when it has reported
    nothing or has ended (`idle`).
- `ready`: errands that ended while nobody held the line, whose outcome waits in
  this session's inbox. The same entries the card's inbox mark counts.
  - `target`: as in `open`.
  - `status`: how the errand ended, as `lich wait` reports it: `answered`,
    `unanswered`, `unread`, `undelivered`, `stopped` or `expired`.

An errand sent by a subagent or a workflow step inside the session (a private
ticket, which only a wait on that ticket collects) is left out of `open` and
`ready`: the session's own collect never reaches it, so counting it would show
work the person cannot pick up. An errand handed **to** the session is in
`owed` whoever sent it.

Both sides test against the response in
[`fixtures/mod-status.json`](fixtures/mod-status.json).

## Event → read mapping

| Claude Code mod  | Codex hook           | Antigravity hook     | opencode event       | oh-my-pi event       | Crush hook           | Cursor CLI hook      | Kiro CLI hook        |
|------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|
| status line      | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) |

The mod decides when to read. lich pushes nothing: an errand that opens or
closes shows at the next read.

## lich server side

- **Endpoint** (`internal/terminal/modstatus.go`, `transport.modStatus`):
  validates the token and `session_id`, notes the plugin release, and answers
  with what the relay reports for that session.
- **Relay** (`internal/relay/status.go`, `Service.Status`): reads the open
  tickets and the inbox under the relay's lock. It drains nothing and closes no
  errand; like every other read of the relay it first sweeps what outlived its
  hour, so an expired ticket is never listed as open.

## Client rules (the mod)

- Missing env vars → never read.
- A non-interactive run (`claude -p`) never reads.
- Send `X-Lich-Plugin` on every read.
- Read at most once every few seconds and draw the last reading in between: a
  status line redraws far more often than an errand moves.
- Never retry a failed read; draw nothing from lich until the next one works.
- A `404` means this lich predates the contract: stop reading for the rest of
  the session.

## Known ceilings

- **Claude Code only.** Mods are a Claude Code feature; on every other harness
  the card's tooltip and inbox mark are the only place these errands show.
- **Mods only run where Claude Code turns them on**, from 2.1.280, behind a
  rollout flag and the folder-trust prompt: see mod-control's ceilings.
- **Labels are the ones at send time.** A session renamed after the errand
  opened still shows under its old label here, as it does in the message the
  errand typed.
- **Polled, not pushed.** An answer that lands between two reads shows at the
  next one; the nudge typed at the prompt is still what tells the agent.
