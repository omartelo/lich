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

## Event → action mapping

| Claude Code hook                        | action                        |
|-----------------------------------------|-------------------------------|
| `PostToolUse` (file-mutating tools)     | refresh the session's git status |

Fire it from `PostToolUse` **only for tools that write to disk** — match
`Edit`, `Write`, `NotebookEdit`, `Bash` (and any others that mutate files). Do
**not** fire on read-only tools (`Read`, `Grep`, `Glob`): a git-status refresh
per read would cost more than the poll it is meant to beat. The tool name is on
the hook's stdin payload if a single script filters instead of per-tool matchers.

## lich server side

- **Endpoint** — `internal/terminal/transport.go`, `transport.sessionTouched`:
  validates the token and body (`parseSessionTouched`), then forwards the
  session id.
- **UI push** — `internal/terminal/terminal.go`: emits the global app event
  `session-touched` (`{id}`).
- **Refresh** — `frontend/src/lib/projects.tsx`: resolves the session id to the
  path its card watches (its worktree, else the project path) and calls
  `refreshGitStatus(path)` (`frontend/src/lib/useGitStatus.ts`), which fetches
  that path now, ahead of the poll tick. A no-op when no card watches the path
  (the session lives in a background project), so it costs no git call.

## Known ceilings

- **Poll stays the baseline.** This never replaces the ~3s poll (see the git
  status ceiling in the repo `CLAUDE.md`); it only front-runs it. Changes from
  outside Claude (a shell session, an external editor) are still caught by the
  poll, not this hook.
- **No debounce.** A burst of edits fires one POST per tool, each an immediate
  git fetch. Cheap and idempotent (unchanged status short-circuits re-renders),
  but a trailing debounce could collapse bursts if it ever matters.
- **Session id, not path.** The hook sends the session id and lich/the frontend
  resolve the path, so a `cd` inside a Bash tool can't point the refresh at the
  wrong repository.
