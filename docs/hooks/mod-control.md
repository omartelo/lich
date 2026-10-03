# Contract: mod control

Lets lich drive a Claude Code session from its card: start a turn with a prompt,
stop the running turn, override the model or the effort level of each request,
and compact the context. The client is a Claude Code **mod** (Claude Code 2.1.287
or later), a plugin whose hooks run inside the Claude Code process, shipped by
the companion plugin. It is not a hook script: instead of reporting events, it
holds a long poll open and applies the commands lich hands it, then reports how
each one went.

This replaces nothing. The other contracts keep reporting the session's state,
and a prompt sent here surfaces in Claude Code as a message from the plugin
rather than as text typed at the PTY.

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
nobody will collect. A session whose process has exited or been closed is
refused the same way, whatever polled last. A command still queued 30 seconds
after it was issued (its mod stopped polling) is dropped at the next poll
rather than handed to a mod that comes back later, when it would land on an
unrelated turn.

### Commands

```
{"id": "m1", "kind": "prompt",  "text": "run the tests"}
{"id": "m2", "kind": "abort"}
{"id": "m3", "kind": "model",   "model": "claude-opus-4-1"}
{"id": "m4", "kind": "model"}
{"id": "m5", "kind": "effort",  "effort": "high"}
{"id": "m6", "kind": "effort"}
{"id": "m7", "kind": "compact", "instructions": "keep the test plan"}
{"id": "m8", "kind": "compact"}
```

- `id`: opaque, unique within one lich process. Echo it in the ack.
- `kind`: one of `prompt`, `abort`, `model`, `effort`, `compact`. A kind the
  mod does not know is acked `ok: false` with `error: "unknown kind"`, so a lich
  newer than the mod is told rather than ignored.
- `text`: `prompt` only and never empty, the message that starts a turn.
- `model`: `model` only. Present: every request from now on uses this model.
  Absent: drop the override and go back to the session's own model.
- `effort`: `effort` only, with the same present/absent rule as `model`.
- `instructions`: `compact` only and optional, passed to the compaction as its
  instructions.

A field that does not belong to the kind is absent: lich refuses to queue a
command that carries one. lich does not validate
model names or effort levels; the mod passes them on and acks what Claude Code
made of them.

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
- `kind`: required, the command's `kind`, echoed. One of the five kinds; any
  other value is refused.
- `ok`: a boolean. `true` once the command took effect, `false` when it could
  not be applied. Missing reads as `false`.
- `error`: optional, why a command failed, trimmed and cut to 120 characters.

Responses: `204` ok · `401` invalid token · `400` invalid body.

lich keeps no table of the commands it handed out, so an ack for an id it does
not know (one from a previous lich process) is accepted like any other.

Both sides test against the payloads in
[`fixtures/mod-control.jsonl`](fixtures/mod-control.jsonl).

## Command → action mapping

| Claude Code mod                         | Codex hook             | Antigravity hook       | opencode event         | oh-my-pi event         | Crush hook             | Cursor CLI hook        | Kiro CLI hook          | ack `ok: true` means                       |
|-----------------------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|------------------------|--------------------------------------------|
| `prompt` → `$.prompt.submit`            | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the turn was submitted                     |
| `abort` → `$.turn.abort`                | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the running turn ended; lich ends it too   |
| `model` → `turn.step` model override    | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the override is set (or dropped)           |
| `effort` → `turn.step` effort override  | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the override is set (or dropped)           |
| `compact` → `$.session.compact`         | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | none (no mod system)   | the compaction ran                         |

Mods are a Claude Code feature, so this contract has one client. Every other
harness never polls, and lich refuses to queue a command for it as detached.

**An abort is the one ack lich acts on.** `$.turn.abort` ends the turn without
firing Claude Code's `Stop` hook, so the [session-state](session-state.md)
report that would end the turn never comes and the card would spin forever.
An `ok: true` ack of an `abort` ends the turn lich has open for the session the
way a `Ctrl+C` typed at the PTY does: the card reads `interrupted`, not `done`.
A failed ack, or an abort while lich has no turn open, changes nothing.

`compact` re-fires Claude Code's `SessionStart` hook, which reports through
[session-start](session-start.md) as usual.

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

## lich server side

- **Fetch** (`internal/terminal/modcontrol.go`, `transport.modCommands`):
  validates the method, token and `session_id`, records the plugin release, then
  waits on the session's queue (`modQueue.take`) and answers with what it
  drained, minus any command older than the attach window. A poll whose client
  is gone by the time it reaches the queue drains nothing; one whose client
  leaves after the drain loses what it drained (see At most once).
- **Ack** (`transport.modAck`): validates the token and body (`parseModAck`).
  A failed command is logged; an `ok: true` `abort` calls
  `Service.noteInterrupt`, which emits `session-status` with the state
  `interrupted` and closes the turn's snapshot window, exactly as for an
  interrupt typed at the PTY.
- **Queue** (`Service.EnqueueModCommand`): the one producer. It refuses an
  unknown kind, a field of another kind, an empty prompt, and a session with
  no running process or no mod polling (`errModDetached`), and otherwise
  returns the id the ack will carry. Nothing
  calls it yet; the card's controls and any RPC or MCP surface come later.
- **Teardown**: a session's queue is dropped when its process exits, and when
  it is closed once the process is gone (a live mod re-polls at once and would
  attach again). Either releases a parked poll with `[]`.

## Known ceilings

- **At most once.** A command is gone from the queue the moment a response
  carries it. A response lost on the way (the mod reloaded, the connection
  dropped after lich wrote it) loses the command, and lich never resends one: a
  prompt or an abort delivered twice is worse than one delivered never. The ack
  is the only receipt.
- **An abort ends the turn only through its ack.** If the ack never arrives,
  the card keeps spinning until the next state report, as it did before this
  contract. And it ends it as `interrupted`, never `done`.
- **`effort` is unmeasured.** The model override was confirmed in the
  transcript on Claude Code 2.1.288; the effort override was not.
- **Whether `UserPromptSubmit` fires for `$.prompt.submit` is unmeasured.** If it
  does not, a prompt sent here starts a turn the card never hears open. The fix
  then belongs in the ack, an `ok: true` `prompt` reporting `busy`, with no new
  endpoint.
- **The `$.http.fetch` timeout is unmeasured.** The 25 second wait has to stay
  below it, or every poll ends as a client error and the mod backs off.
- **A mod reload may strand a parked poll.** Whether a hot reload cancels the
  old mod's fetch is unmeasured; if it does not, a command can drain into a
  JavaScript context that no longer exists.
- **Overrides live in the mod.** A model or effort override is the mod's state,
  so a mod reload drops it without telling lich.
- **Commands queued when lich exits are dropped.** The queue is in memory.
