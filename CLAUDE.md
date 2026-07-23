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
- **Frontend**: React 18 + TypeScript + Vite. Terminal: xterm.js 6 + WebGL addon. Code/Review dock: CodeMirror.
  Sessions spawn any registered provider (`internal/providers`: Claude Code, Codex, OpenCode, Crush). Service shapes
  are hand-owned in `frontend/src/lib/api-types.ts` (mirrors of the Go structs' JSON tags — keep in sync).
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

Frontend in isolation: `cd frontend && pnpm run build` (runs `tsc` + `vite build`).

## Local Gate (before every commit / PR)

- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `go test ./...` (backend) and `cd frontend && pnpm test` (frontend) green — or `task test` for both.
- `cd frontend && pnpm build` succeeds (tsc typecheck + vite).
- Touched an OS seam or a `_test.go` build tag? Run the same cross-compile loop CI runs:
  `for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done`

CI (`.github/workflows/ci.yml`) runs this gate on every PR and push to `main` and renders pass/fail counts plus
coverage into the job summary. The summary is transparency, never the gate — a red test fails the job.

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

Push a `vX.Y.Z` tag. `.github/workflows/release.yml` fans out into three parallel build jobs — `linux` (packages),
`windows` (exe + Inno Setup installer), `mac` (both-arch raw binaries), each running the backend suite on its own
OS — then `release` fans in (one `checksums.txt`, notes from the matching `CHANGELOG.md` section) and `aur`
publishes `lich-bin`. A `workflow_dispatch` run from any branch exercises the whole pipeline without publishing.
The version comes from the git tag (`git describe` in the Taskfile, env `VERSION` overrides) and is injected into
`build/linux/nfpm/nfpm.yaml`.

Before tagging:

- [ ] Local gate green, backend with `-race`.
- [ ] Move `CHANGELOG.md`'s `[Unreleased]` entries under a new `vX.Y.Z` heading and refresh the compare links.
- [ ] Tag `vX.Y.Z` and push.

## Known Ceilings

Deliberate limits and shortcuts — one line of *what and where*; the mechanism and its history live in the code and
`CHANGELOG.md`.

- **Session cwd is polled** from the PTY child (`internal/terminal/cwd.go`, per-OS readers behind build tags); a
  failed read degrades to the session's start directory. Tracks the direct child only, not nested shells.
- **git status is polled** — one shared poller per repository path (`frontend/src/lib/git-status-store.ts`); the
  lich plugin's `session-touched` hook nudges an immediate refresh. An fs watcher is the upgrade path.
- **Persistence is hybrid**: UI prefs in the page's localStorage (`lich.*` keys — the reason the listener port is
  pinned at 47821; `LICH_LISTEN_PORT` overrides it, `LICH_PORT` is the distinct per-session hook variable), the
  workspace in SQLite (`<config-dir>/lich/lich.db`, `internal/store`). Closing a session deletes its row; keeping a
  worktree parks its session for a later resume; a closed project is hidden, never deleted.
- **Hidden sessions are serialized and destroyed**: 2MB replay rings on both sides (`frontend/src/lib/replay-buffer.ts`
  page-side, `internal/terminal/replay.go` backend-side — the latter survives a full page reload). waveterm's disk
  filestore is the upgrade path if size ever matters.
- **Terminal I/O rides the loopback WebSocket**; when the socket is down, output falls back to `/events` and input
  to the RPC — slower, never broken.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch focuses
  the running window (best-effort, untested against a real window) and exits 0.
- **Self-update** (`internal/appupdate`): checks at startup, then hourly. Self-apply is Windows/macOS only; Linux
  pastes a distro-specific command (AUR `lich-bin` on Arch, `install.sh` elsewhere) and relaunches through
  `/restart` (`internal/restart`).
- **Reordering rides dnd-kit's pointer sensors** (`frontend/src/lib/use-sortable-list.ts`); the activation distance
  is load-bearing — without it plain clicks stop selecting a session.
- **No AppImage** — deliberate (`docs/chromium-shell.md`; CEF is the bundling path if it ever matters). Ships
  `.deb`/`.rpm`/`.pkg.tar.zst`, `install.sh`, and AUR `lich-bin`.
- **Windows is experimental**: cmd.exe shell, GUI subsystem build — no console, diagnostics in
  `%AppData%\lich\lich.log`. Missing: code signing, winget manifest, Windows PTY tests.
- **macOS is experimental**: unsigned raw binaries only (Gatekeeper quarantines; no `.dmg`/Homebrew yet); the window
  path has not been smoke-tested on real hardware. Only darwin-specific code: the browser candidates
  (`internal/chromium`).
