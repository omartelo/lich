# Contract: session touched

Signals that a session likely changed files on disk, so lich refreshes that
session's git status **immediately** instead of waiting for its steady poll. It
is a latency optimization, not a source of truth: lich polls git every ~3s
regardless, so a user without the plugin sees the same diff badge — just up to a
poll interval later.

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-touched?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>"}
```

Responses: `204` ok · `401` invalid token · `400` invalid body.

Both sides test against the payloads in
[`fixtures/session-touched.jsonl`](fixtures/session-touched.jsonl).

## Event → action mapping

| Claude Code hook                        | Codex hook                          | opencode event | oh-my-pi event                       | Crush hook                          | action                           |
|-----------------------------------------|-------------------------------------|----------------|--------------------------------------|-------------------------------------|----------------------------------|
| `PostToolUse` (file-mutating tools)     | `PostToolUse` (file-mutating tools) | `file.edited`  | `tool_result` (file-mutating tools)  | `PreToolUse` (file-mutating tools)  | refresh the session's git status |

Fire it from `PostToolUse` **only for tools that write to disk** — the names are
the provider's, so match its own: `Edit`, `Write`, `NotebookEdit`, `Bash` on
Claude Code, `apply_patch` (plus `Bash`) on Codex, and the same set lower-cased
(`edit`, `write`, `multiedit`, `bash`) on Crush. Do **not** fire on read-only
tools (`Read`, `Grep`, `Glob`, Crush's `view`): a git-status refresh per read
would cost more than the poll it is meant to beat. The tool name is on the
hook's stdin payload if a single script filters instead of per-tool matchers.

opencode needs no matcher: `file.edited` fires only when a file actually
changed, which is the signal the other harnesses approximate by filtering tool
names. oh-my-pi filters like Claude Code does, on the result rather than the
call so the refresh sees the tree after the write; its `python` tool is left out
on purpose, being a general evaluator that would fire a refresh per
computation. Crush is the opposite — `PreToolUse` fires *before* the write, so its
refresh reads a tree the tool has not touched yet (see the ceilings).

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionTouched`:
  validates the token and body (`parseSessionTouched`), then forwards the
  session id.
- **UI push** — `internal/terminal/terminal.go`: emits the global app event
  `session-touched` (`{id}`).
- **Refresh** — `frontend/src/providers/projects.tsx`: resolves the session id to the
  path its card watches (its worktree, else the project path) and calls
  `refreshGitStatus(path)` (`frontend/src/lib/git/use-git-status.ts`), which fetches
  that path now, ahead of the poll tick. A no-op when no card watches the path
  (the session lives in a background project), so it costs no git call.

## Known ceilings

- **Poll stays the baseline.** This never replaces the ~3s poll (see the git
  status ceiling in `docs/ceilings.md`); it only front-runs it. Changes from
  outside Claude (a shell session, an external editor) are still caught by the
  poll, not this hook.
- **No debounce.** A burst of edits fires one POST per tool, each an immediate
  git fetch. Cheap and idempotent (unchanged status short-circuits re-renders),
  but a trailing debounce could collapse bursts if it ever matters.
- **Session id, not path.** The hook sends the session id and lich/the frontend
  resolve the path, so a `cd` inside a Bash tool can't point the refresh at the
  wrong repository.
- **On Crush the refresh runs one tool early.** `PreToolUse` is the only event
  Crush has, so the report fires before the write lands and the immediate fetch
  sees the tree as it was. What it actually front-runs is the *previous* tool's
  write, and the last write of a turn waits for the poll like it always did.
  Registering it anyway costs one git call per write tool and makes a burst of
  edits show up a poll earlier than nothing would; a `PostToolUse` in Crush
  turns this into the same signal the other three send.
