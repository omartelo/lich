# lich

## What it is

`lich` is a **personal harness** — a desktop app whose Go backend serves an embedded React frontend to a system
Chromium window in `--app` mode (no Electron, no webview toolkit; decision record: `docs/chromium-shell.md`). It is
a bespoke tool, not a generic product. Linux first; Windows and macOS builds are experimental (see Known Ceilings).

This file records only what the code cannot say: invariants, deliberate ceilings, and the workflow. User-facing
feature history lives in `CHANGELOG.md` — don't duplicate it here.

## Stack

- **Backend**: Go 1.25, pure Go (CGO_ENABLED=0, fully static binary). One token-authenticated loopback listener
  carries everything: HTTP RPC (`internal/rpc`) plus WebSockets for terminal I/O (`internal/terminal/transport.go`)
  and app events (`internal/events`). OS-specific code is selected by build tags behind small seams, never by
  runtime checks — the PTY is the model (`internal/terminal`: creack/pty on Unix, ConPTY on Windows).
- **Shell**: system Chromium-family browser launched in `--app` mode (`internal/chromium`), persistent profile under
  the user config dir (`os.UserConfigDir` + `lich/chromium-profile`). Window closed = app exit.
- **Frontend**: React 18 + TypeScript + Vite; the UI rules and the visual system are `frontend/CLAUDE.md` and
  `frontend/DESIGN.md`. Sessions spawn any registered provider (`internal/providers`: Claude Code, Codex, OpenCode,
  Crush). Service shapes are hand-owned in `frontend/src/lib/api-types.ts` — touch a Go struct's JSON tags and that
  mirror moves in the same change; there is no codegen.
- **Session hooks**: `docs/hooks/` is the canonical, contract-first spec. lich owns the server side; the scripts
  live in the companion repo `omartelo/lich-plugin`, so an endpoint or payload change breaks a repo this one cannot
  see — move the contract first, then both sides.
- **Build/tasks**: [Task](https://taskfile.dev) — see Commands.

## Commands

```bash
task dev              # dev mode: Vite HMR + backend; separate DB, port and Chromium profile
task build            # frontend build + static Go binary → bin/lich
task build:windows    # cross-compiles bin/lich.exe (experimental)
task build:mac        # cross-compiles bin/lich-darwin-{arm64,amd64} (experimental)
task run              # build + run
task test             # go test ./... + frontend vitest
task mutation         # mutation testing via gremlins (scope: task mutation -- ./internal/store/)
task package          # .deb, .rpm, .pkg.tar.zst into bin/ (needs nfpm)
task package:windows  # bin/lich-setup.exe installer (needs Inno Setup 6 — CI/Windows)
```

## Local Gate (before every commit / PR)

- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `cd frontend && pnpm check` clean — biome is the frontend's gofmt + vet (fix with `pnpm format`).
- `go test ./...` (backend) and `cd frontend && pnpm test` (frontend) green — or `task test` for both.
- `cd frontend && pnpm build` succeeds (tsc typecheck + vite).
- Shipped anything a user can see? Its `CHANGELOG.md` `[Unreleased]` entry lands in the same PR — the release notes
  are read from there, so an entry written later is an entry that missed its release.
- Touched an OS seam or a `_test.go` build tag? Run the same cross-compile loop CI runs:
  `for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done`
  Cross-compiling only proves it builds — label the PR `ci:os` to run the backend suite on real Windows and macOS
  runners before the merge.

CI (`.github/workflows/ci.yml`) runs this gate on every PR and push to `main` and renders pass/fail counts plus
coverage into the job summary. The summary is transparency, never the gate — a red test fails the job. The
`os-tests` job runs the backend suite on Windows and macOS: always on a push to `main` (so a regression is caught
before a tag), on a PR only when it carries the `ci:os` label.

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Test coverage ≥ 80%**, backend and frontend. CI measures and reports the number but does not auto-fail below
   the bar — it is held in review, so read the summary. OS/framework boundaries (the PTY, the Chromium
   launcher/zenity subprocesses, WebSocket wiring, the `main` bootstrap, xterm.js internals) are the documented
   exception: cover the pure logic and leave the boundary itself alone.
2. **Tests answer to the contract, never the other way round.** Never weaken, skip, delete or rewrite a test to buy
   a green run or a coverage number. A test may change for exactly two reasons: the contract changed, or the test
   asserted something the contract never promised — name which one in the diff. When in doubt, change the product,
   not the test.
3. **A flake is a bug, and it has a root cause.** Never re-run CI until it goes green, never dismiss a failure as
   "unrelated" without proving it. Reproduce (`go test -count=200 -run TestX ./pkg/`), fix the cause, and measure
   the failure rate before and after.
4. **Clean code.** Small focused functions (< 50 lines); cohesive files (200–400 lines typical, 800 max); no
   nesting deeper than 4 levels; comments only for the *why*; errors handled explicitly, never swallowed; no magic
   values; no secrets in source.

## Releases

Push a `vX.Y.Z` tag — `.github/workflows/release.yml` builds every OS, runs the backend suite on each, publishes the
artifacts and the AUR `lich-bin`, and takes the release notes from the matching `CHANGELOG.md` section. A
`workflow_dispatch` run from any branch exercises the whole pipeline without publishing. The version comes from the
git tag (`git describe` in the Taskfile, env `VERSION` overrides) and is injected into `build/linux/nfpm/nfpm.yaml`
— never hand-edit it there.

Before tagging:

- [ ] Local gate green, backend with `-race`.
- [ ] Move `CHANGELOG.md`'s `[Unreleased]` entries under a new `vX.Y.Z` heading and refresh the compare links.
- [ ] Tag `vX.Y.Z` and push.

## Known Ceilings

Deliberate limits and shortcuts. A bullet earns its place by naming a trap — something that breaks work when
nobody knows it and that the call site never shows. The mechanism and the history stay in the code and
`CHANGELOG.md`.

- **Session cwd is polled** from the PTY child (`internal/terminal/cwd.go`, per-OS readers behind build tags); a
  failed read degrades to the session's start directory. Tracks the direct child only, not nested shells.
- **A project's gh account governs gh, not git**: `vcs.account` (`internal/project/ghaccount.go`) puts one
  account's token in `GH_TOKEN` for every gh call lich makes for that project. A push still rides the remote's
  ssh key and signs with the global `user.email`, so a PR can be *read* by one account and its commits *land*
  under another, with no error anywhere. Per-worktree `core.sshCommand` + `user.email` is the upgrade path.
- **The cost readout is priced from a table, not from the provider**: no provider publishes an API for what a
  turn cost, so `internal/pricing` bills the transcript's token counts against a baked table that refreshes
  itself from LiteLLM's published one when it meets an unknown model. Two consequences: a model nobody has
  priced yet makes the readout go *absent* (the scan stops at that line — a total missing a turn is worse than
  no total), and the accounting is per `(session, transcript)` in `session_costs`, so a conversation forked
  inside the PTY — which copies its history into a new transcript — bills that history twice. lich's own
  resume continues the same transcript and is unaffected.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh. An fs watcher is the upgrade path.
- **Persistence is hybrid**: UI prefs in the page's localStorage (`lich.*` keys — the reason the listener port is
  pinned at 47821; `LICH_LISTEN_PORT` overrides it, `LICH_PORT` is the distinct per-session hook variable), the
  workspace in SQLite (`<config-dir>/lich/lich.db`, `internal/store`). Closing a session deletes its row; keeping a
  worktree parks its session for a later resume; a closed project is hidden, never deleted.
- **Hidden sessions are serialized and destroyed**: 2MB replay rings on both sides (`frontend/src/lib/terminal/replay-buffer.ts`
  page-side, `internal/terminal/replay.go` backend-side — the latter survives a full page reload). waveterm's disk
  filestore is the upgrade path if size ever matters.
- **Terminal I/O degrades, never breaks**: with the WebSocket down, output falls back to `/events` and input to the
  RPC — slower, still working.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch focuses
  the running window (best-effort, untested against a real window) and exits 0.
- **Reordering rides dnd-kit's pointer sensors** (`frontend/src/lib/use-sortable-list.ts`); the activation distance
  is load-bearing — without it plain clicks stop selecting a session.
- **Windows is experimental**: cmd.exe is the shell, and the GUI subsystem build has no console — diagnostics only
  reach `%AppData%\lich\lich.log`. The PTY has no Windows tests.
- **macOS is experimental**: the window path has never run on real hardware, and the binaries are unsigned, so
  Gatekeeper quarantines them. Only darwin-specific code: the browser candidates (`internal/chromium`).
