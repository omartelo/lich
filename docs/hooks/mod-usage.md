# Contract: mod usage

Lets a Claude Code session report its own context window, rate limits and cost,
the figures its status line draws, so lich shows those instead of deriving them.
The client is the same Claude Code **mod** as [mod-control.md](mod-control.md)
(Claude Code 2.1.280 or later, shipped by the companion plugin), in a module of
its own: it hooks `session.measure` and posts what Claude Code measured.

This replaces nothing on the wire. Every figure it carries still has the
fallback lich used before it, for a session whose plugin predates this contract
and for every other harness.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/mod/usage?token=${LICH_TOKEN}
Content-Type: application/json
X-Lich-Plugin: <plugin release>

{
  "session_id": "<LICH_SESSION_ID>",
  "conversation_id": "3fb3ad05-5b58-45ea-a1e0-0d1d4b0a89b9",
  "context": {"window": 200000, "tokens": 43592, "percent": 22},
  "rate_limits": [
    {"kind": "five_hour", "percent_used": 14, "resets_at": "2026-10-05T00:30:00.000Z"},
    {"kind": "seven_day", "percent_used": 80, "resets_at": "2026-10-05T15:00:00.000Z"}
  ],
  "cost_usd": 0.0977631
}
```

- `session_id`: required, `LICH_SESSION_ID`.
- `conversation_id`: required, Claude Code's session id (`$.session.id()`), the
  id [session-start](session-start.md) reports. Every figure below belongs to
  that conversation, and lich applies none of them to another one.
- `context.window`: required, a positive integer, the context window of the
  session's model in tokens.
- `context.tokens`: optional, a non-negative integer, the input tokens the last
  response was answered over. Absent before the conversation's first response.
- `context.percent`: optional, an integer from 0 to 100, Claude Code's own
  `tokens` over `window`, rounded.
- `rate_limits`: optional, the account's rate-limit windows, empty or absent off
  a subscription. Each has a non-empty `kind` (`five_hour`, `seven_day`, or a
  gateway's `spend_limit`), a non-negative `percent_used` (past 100 on an
  exceeded spend limit) and an optional RFC 3339 `resets_at`.
- `cost_usd`: optional, a non-negative number, what the conversation has cost
  so far in US dollars, as `/cost` totals it. Absent where Claude Code keeps no
  ledger.

Responses: `204` ok · `401` invalid token · `400` invalid body · `405` not a
POST.

Both sides test against the payloads in
[`fixtures/mod-usage.jsonl`](fixtures/mod-usage.jsonl).

## Event → report mapping

| Claude Code mod      | Codex hook           | Antigravity hook     | opencode event       | oh-my-pi event       | Crush hook           | Cursor CLI hook      | Kiro CLI hook        |
|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|----------------------|
| `session.measure`    | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) | none (no mod system) |

`session.measure` fires once when the session starts (window and rate limits,
no tokens yet), once at the end of every main-thread turn, and when a rate-limit
window moves a whole point. A `/clear` fires it at once under the new
conversation id; a resume or a fork fires it at start with the conversation's
whole figures. Measured on Claude Code 2.1.289.

## lich server side

- **Endpoint** (`internal/terminal/modusage.go`, `transport.modUsage`):
  validates the token and body (`parseModUsage`) and hands the report to the
  service, which keeps the latest one per session and pushes the footer's usage
  event again.
- **Context window**: for the conversation a report names, the window is the
  reported one, which replaces the guess from the model's name. The tokens are
  still read from the transcript, which updates mid-turn and after a compaction
  where no report fires; when they equal the reported tokens, the percentage is
  Claude Code's own.
- **Cost**: a reported `cost_usd` is that conversation's whole cost. It replaces
  the transcript scan for it, sub-agents included, in the same ledger row the
  scan would write, so totals, `/clear` and forks add up as before.
- **Rate limits**: `five_hour` and `seven_day` feed the plan gauge of a session
  whose login is a long-lived token (`CLAUDE_CODE_OAUTH_TOKEN`), in place of the
  request lich would spend to measure it. A reading counts until one of its
  windows resets; after that, or without a report, lich measures as before. A
  login that reaches the usage route keeps reading it, since that route names
  the plan, the account and the model-scoped caps a report does not carry.
  `spend_limit` is not drawn.

## Client rules (the mod)

- Missing env vars → never report.
- A non-interactive run (`claude -p`) never reports: one started inside a lich
  session inherits its `LICH_*` and would report onto the card that started it.
- Send `X-Lich-Plugin` on every report.
- Report every `session.measure`, and never hold the event up on lich: post and
  pass the event on, drop a report that fails, never retry one.
- A `404` means this lich predates the contract: stop reporting for the rest of
  the session.

## Known ceilings

- **Claude Code only.** Mods are a Claude Code feature, and no other harness
  runs anything that measures itself; the rest keep the readouts lich derives.
- **Mods only run where Claude Code turns them on**, from 2.1.280, behind a
  rollout flag and the folder-trust prompt: see mod-control's ceilings. The
  module ships in lich-plugin 0.16.0. A session whose mod is off, or whose
  plugin is older, reports nothing and reads as before.
- **Nothing fires mid-turn or on a compaction.** The tokens stay the
  transcript's for that reason; a cost is the one at the last turn's end until
  the next one ends.
- **Rate limits move only with this session's responses.** Spend from elsewhere
  on the same account shows when the session next answers, which is what Claude
  Code's own status line shows.
- **A fork's reported cost includes the history it copied.** lich offsets it the
  way it offsets its own scan, by the parent's ledger at the fork.
