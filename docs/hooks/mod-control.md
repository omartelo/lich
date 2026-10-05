# Contract: mod control

Lets lich drive a Claude Code session (`lich control`, the `control_session` MCP
tool): start a turn with a prompt, stop the running turn, override the model or
the effort level of each request, and run one of its slash commands; and ask it
a side question it answers without stopping (`lich ask`, the `ask_session` MCP
tool). The client is a Claude Code **mod** (Claude Code 2.1.280
or later), a plugin whose hooks run inside the Claude Code process, shipped by
the companion plugin. It is not a hook script: instead of reporting events, it
holds a long poll open and applies the commands lich hands it, then reports how
each one went.

It also carries lich's relay to a Claude Code session: a task sent with
`lich send` or `send_to_session`, the note that results are ready and a
scheduled prompt arrive as a `prompt` when the mod is attached, and are typed at
the PTY otherwise. The other contracts keep reporting the session's state, and a
prompt sent here surfaces in Claude Code as a message from the plugin rather
than as text typed at the PTY.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows; the rules below
add to them for a client that stays running.

## Request: fetch commands

```
GET http://127.0.0.1:${LICH_PORT}/mod/commands?token=${LICH_TOKEN}&session_id=${LICH_SESSION_ID}
X-Lich-Plugin: <plugin release>
```

lich answers at once when commands are queued for the session, otherwise it
holds the request up to 25 seconds and answers when one is queued or the wait
runs out. The body is always a JSON array, `[]` when nothing arrived:

```
200 OK
Content-Type: application/json

[{"id":"m7","kind":"abort"}]
```

Responses: `200` the commands, oldest first · `401` invalid token · `400`
missing `session_id` · `405` not a GET.

A response carries every command queued for the session and removes them from
the queue: delivery is **at most once**. A command lich handed to a poll whose
connection then dropped is lost, never sent twice.

A poll is also what attaches the mod. lich refuses to queue a command for a
session whose mod has not polled within the last 30 seconds (the 25 second wait
plus a 5 second grace for the next poll to arrive), so a session without the mod
is told so when the command is issued, not left waiting for a command that
nobody will collect. A session whose process has exited, been closed or never
started is refused as not running, whatever polled last. A command still queued
30 seconds after it was issued (its mod stopped polling) is dropped at the next
poll rather than handed to a mod that comes back later, when it would land on an
unrelated turn. A command whose caller stopped waiting before any poll collected
it is withdrawn at once, so the caller is never told it is still coming.

### Commands

```
{"id": "m1", "kind": "prompt",  "text": "run the tests"}
{"id": "m2", "kind": "abort"}
{"id": "m3", "kind": "model",   "model": "claude-opus-4-1"}
{"id": "m4", "kind": "model"}
{"id": "m5", "kind": "effort",  "effort": "high"}
{"id": "m6", "kind": "effort"}
{"id": "m7", "kind": "command", "name": "compact", "args": "keep the test plan"}
{"id": "m8", "kind": "command", "name": "clear"}
{"id": "m9", "kind": "ask",     "question": "what are you working on?"}
```

- `id`: opaque, unique within one lich process. Echo it in the ack.
- `kind`: one of `prompt`, `abort`, `model`, `effort`, `command`, `ask`. A kind the
  mod does not know is acked `ok: false` with `error: "unknown kind"`, so a lich
  newer than the mod is told rather than ignored.
- `text`: `prompt` only and never empty, the message that starts a turn.
- `model`: `model` only. Present: every request from now on uses this model.
  Absent: drop the override and go back to the session's own model.
- `effort`: `effort` only, with the same present/absent rule as `model`.
- `name`: `command` only and required, the slash command's name without the
  slash.
- `args`: `command` only and optional, what follows the name, as typed.
- `question`: `ask` only and never empty, at most 8 KiB.

A field that does not belong to the kind is absent: lich refuses to queue a
command that carries one. lich does not validate
model names or effort levels. The mod refuses an effort outside `low`, `medium`,
`high`, `xhigh` and `max`, and stores a model name unchecked.

lich drops one leading `/` from `name`. It refuses a `command` named `model` or
`effort`, in any case and with or without the slash: run through a mod, those
save the value as the default for every new session, and the `model` and
`effort` kinds are the per-session route. lich does not check that a command
exists; the mod acks an unknown name `ok: false` with Claude Code's own error.

Both sides test against the response shapes in
[`fixtures/mod-commands.json`](fixtures/mod-commands.json).

## Request: acknowledge a command

```
POST http://127.0.0.1:${LICH_PORT}/mod/acks?token=${LICH_TOKEN}
Content-Type: application/json
X-Lich-Plugin: <plugin release>

{"session_id": "<LICH_SESSION_ID>", "id": "m7", "kind": "abort", "ok": true}
```

- `session_id`: required, `LICH_SESSION_ID`.
- `id`: required, the command's `id`.
- `kind`: required, the command's `kind`, echoed. One of the six kinds; any
  other value is refused.
- `ok`: a boolean. `true` once the command took effect, `false` when it could
  not be applied. Missing reads as `false`.
- `error`: optional, why a command failed, trimmed and cut to 120 characters.
  An `ask` that got no answer carries the fork's reason: `nothing-to-fork`,
  `api-error <status> <kind>`, `empty-reply` or `aborted`.
- `answer`: an answered `ask` only (`ok: true`), the reply's text, which the mod
  cuts at 16,000 UTF-16 units and marks `[truncated]`. On any other ack it is
  refused.

The body is held to 64 KiB, the room an answer needs, where every other hook
endpoint takes 4 KiB.

Responses: `204` ok · `401` invalid token · `400` invalid body.

lich does not resend commands, so an ack for an id nobody is waiting on (one
from a previous lich process, or one whose wait already ended) is accepted like
any other.

Both sides test against the payloads in
[`fixtures/mod-control.jsonl`](fixtures/mod-control.jsonl).

## Command → action mapping

| Claude Code mod                         | Codex hook             | Antigravity hook       | opencode event         | oh-my-pi event         | Crush hook             | Cursor CLI hook        | Kiro CLI hook          | ack `ok: true` means                       |
|-----------------------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|--------------------------------------------|
| `prompt` → `$.prompt.submit`            | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | its turn started                           |
| `abort` → `$.turn.abort`                | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the running turn ended; lich ends it too   |
| `model` → `turn.step` model override    | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the override is set (or dropped)           |
| `effort` → `turn.step` effort override  | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the override is set (or dropped)           |
| `command` → `$.command.run({ command: name, args })` | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the command ran, which is once the session was idle |
| `ask` → `$.model.fork({ prompt })`      | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the session answered; `answer` holds it    |

Mods are a Claude Code feature, so this contract has one client. Every other
harness never polls, and lich refuses to queue a command for it as detached.

**An abort is the one ack lich acts on.** `$.turn.abort` ends the turn without
firing Claude Code's `Stop` hook, so the [session-state](session-state.md)
report that would end the turn never comes and the card would spin forever.
An `ok: true` ack of an `abort` ends the turn lich has open for the session the
way a `Ctrl+C` typed at the PTY does: the card reads `interrupted`, not `done`.
A failed ack, or an abort while lich has no turn open, changes nothing.

A `command` running `/compact` or `/clear` re-fires Claude Code's `SessionStart`
hook, which reports through [session-start](session-start.md) as usual.

An `ask` is a fork: one tool-less request over the session's own transcript as
its main thread last sent it, with the question after it. It runs beside a turn
without touching it, the API's prompt cache serves the transcript, and neither
the question nor the answer enters the conversation. The mod puts the question
behind a preamble saying it is a side question and that tools are unavailable:
without it, a fork made mid-turn reaches for a tool, is refused, and answers in
a second request at twice the latency (measured on 2.1.289).

## Client rules (the mod)

- Missing env vars → never start polling.
- Send `X-Lich-Plugin` on every poll and every ack.
- Re-poll at once after a `200`, empty or not.
- On a network error or a `5xx`, wait 1 second before the next poll, doubling
  up to 10 seconds, and reset after a `200`.
- A `404` means this lich predates the contract: stop polling for the rest of
  the session.
- Apply commands in the order they arrive, ack each one, and never block or
  fail a turn on lich: an ack that cannot be sent is dropped.
- Wait for the answer to an `abort`'s ack, or for that ack to fail, before
  applying the next command. lich ends whatever turn it has open when the ack
  lands, so an ack that arrives after the next `prompt` opened a turn would end
  that turn instead.
- A `prompt` and a `command` settle only once the session is idle. Apply them
  in order, but do not wait for them before applying the next command: an
  `abort` held behind one would reach the running turn only after it ended.
- Answer an `ask` outside that order: it can take a minute and changes nothing
  the other commands depend on. Several may run at once.

## lich server side

- **Fetch** (`internal/terminal/modcontrol.go`, `transport.modCommands`):
  validates the method, token and `session_id`, records the plugin release, then
  waits on the session's queue (`modQueue.take`) and answers with what it
  drained, minus any command older than the attach window. A poll whose client
  is gone by the time it reaches the queue drains nothing; one whose client
  leaves after the drain loses what it drained (see At most once).
- **Ack** (`transport.modAck`): validates the token and body (`parseModAck`),
  read up to `modAckBodyLimit`.
  A failed command is logged; an `ok: true` `abort` calls
  `Service.noteInterrupt`, which emits `session-status` with the state
  `interrupted` and closes the turn's snapshot window, exactly as for an
  interrupt typed at the PTY. Every ack first releases a wait on that command
  (`Service.RunModCommand`), matched by id and session.
- **Queue** (`Service.EnqueueModCommand`): the producer behind `lich control`. It refuses an
  unknown kind, a field of another kind, an empty prompt, a `command` with no
  name or named `model` or `effort`, an `ask` with no question or one over
  `modQuestionLimit`, a session with no running process
  (`errModNotRunning`) and one with no mod polling (`errModDetached`), and
  otherwise returns the id the ack will carry. `Service.RunModCommand` queues the
  same way and waits for the ack until its context ends; a command no poll
  collected by then is withdrawn from the queue. `spawn.Control` calls it behind
  `lich control` and `control_session`, and `spawn.Ask` behind `lich ask` and
  `ask_session`, for 90 seconds; both add the rules about who asks: Claude Code
  only, never the caller's own session. `Service.SubmitPrompt` is the relay's
  producer (`internal/relay`): it queues a `prompt` the same way and hands the
  relay the wait on its ack, and a session with no mod polling is typed at instead.
- **Teardown**: a session's queue is dropped when its process exits, and when
  it is closed once the process is gone (a live mod re-polls at once and would
  attach again). Either releases a parked poll with `[]`, and releases any wait
  on its commands as ended.

## Known ceilings

- **At most once.** A command is gone from the queue the moment a response
  carries it. A response lost on the way (the mod reloaded, the connection
  dropped after lich wrote it) loses the command, and lich never resends one: a
  prompt or an abort delivered twice is worse than one delivered never. The ack
  is the only receipt.
- **An abort ends the turn only through its ack.** If the ack never arrives,
  the card keeps spinning until the next state report, as it did before this
  contract. And it ends it as `interrupted`, never `done`.
- **Mods only run where Claude Code turns them on.** Measured on 2.1.200 to
  2.1.288: before 2.1.280 the module is ignored or skipped, never refused, and
  the plugin's shell hooks keep working. From 2.1.280 on, Claude Code still
  gates installed plugins' modules behind a rollout flag it caches in
  `~/.claude.json`, and running an older Claude Code under the same home can
  leave it off for the next session. A session whose mod is off never polls, so
  lich refuses its commands as detached.
- **A mod loads only once its folder is trusted.** The shell hooks register
  before the trust prompt is answered; the module waits for it.
- **The poll has five seconds of slack.** Claude Code aborts a mod's
  `$.http.fetch` at a fixed 30 seconds (measured on 2.1.288, with no option to
  change it), and lich answers an empty poll at 25.
- **The effort override is dropped for a model that takes none.** Measured on
  2.1.288 on the wire and in the transcript: the override reaches the request,
  except on a model with no effort setting, where Claude Code sends none. The
  `effort` the shell hooks report does not reflect the override; the
  transcript's per-reply `effort` does.
- **A non-interactive run does not poll.** A `claude -p` inside a lich session
  inherits the session's variables, so it would take the parent's commands and
  hold its own exit for up to 25 seconds; the client polls only from an
  interactive session.
- **An override's ack means stored, not accepted.** Claude Code first sees a
  `model` or `effort` override at the session's next request, after the ack, so
  a model name it rejects fails there, inside Claude Code, while lich holds an
  `ok: true`.
- **Overrides live in the mod.** A model or effort override is the mod's state,
  so a mod reload drops it without telling lich.
- **Commands queued when lich exits are dropped.** The queue is in memory.
- **A relayed prompt is typed only when no poll collected it.** A relayed
  prompt a poll collected and nobody acked may still run, so lich never types it
  again: the relay reports it unread instead.
- **A slash command runs only once the session is idle.** `/compact` sent during
  a turn acks after that turn and the compaction. `lich control` reports it as
  done only when both finish within its 60-second wait, and as delivered
  otherwise.
- **A slash command that opens a dialog holds the session's commands until
  someone closes it.** Measured on 2.1.288 and 2.1.289: `/cost` holds Claude
  Code's command queue until Esc is pressed in the terminal, and its ack and
  every later `command` wait behind it. lich cannot tell such a command apart in
  advance. The mod does not wait on a `command`, so `abort`, `model` and
  `effort` still apply. A dialog opens only on an idle session, so there is no
  turn behind it to abort.
- **`/model` and `/effort` are refused as commands.** Measured on 2.1.288: run
  through a mod they write `~/.claude/settings.json` (`model`,
  `modelSettings.<model>.effortLevel`), the default for every new session.
- **A mod older than 0.15.0 does not know `command`.** It acks `ok: false`,
  `unknown kind`, and lich reports "update lich-plugin to 0.15.0 or later".
- **An answer knows the conversation as of the session's last finished model
  response.** A fork is that request again, so a reply being written or a tool
  call running when the question lands is not in it, nor is a prompt whose first
  reply is still being written: the session cannot say what it is doing this
  very second. Measured on 2.1.289.
- **An ask cannot be cancelled.** A fork takes no signal, and an Esc on the
  session's turn does not cut it (measured: it answered in full after the
  turn was interrupted). Once lich stops waiting, at 90 seconds, the fork runs
  on, and is billed, until it ends, and its answer is dropped.
- **A mod reload loses an ask in flight.** The fork dies with the mod's
  environment and nothing acks it, so the caller waits out the 90 seconds.
- **An ask is not counted in the session's cost.** The fork never enters the
  transcript, which is where lich reads cost from.
- **The cache has to still hold the transcript.** A fork is cheap because the
  prompt cache serves the session's prefix (measured: ~55k tokens read for
  under 100 written). After the cache entry lapses on an idle session, or
  after a model override, the fork pays for the whole prefix.
- **An answer is cut at 16,000 characters.** Measured on 2.1.289, a fork has no
  length bound of its own: a 4,000-word request answered 24,800 characters in
  109 seconds. The mod asks for a brief answer.
