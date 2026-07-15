# lich

## What it is

`lich` is a **personal harness** — a desktop app whose Go backend serves an embedded React frontend to a system
Chromium window in `--app` mode (no Electron, no webview toolkit; see `docs/chromium-shell.md` for the full decision
record). It is shaped by the author's experience with and taste for other harnesses on the market; it is not a generic
product, it is a bespoke tool. Linux-only.

## Stack

- **Backend**: Go 1.25, pure Go (CGO_ENABLED=0, fully static binary). Services exposed to the frontend over loopback
  HTTP RPC (`internal/rpc`) + WebSockets (terminal I/O in `internal/terminal/transport.go`, app events in
  `internal/events`), all on one token-authenticated listener.
- **Shell**: system Chromium-family browser launched in `--app` mode (`internal/chromium`), persistent profile under
  `~/.config/lich/chromium-profile`. Window closed = app exit. Runtime needs: chromium/chrome on PATH, zenity for the
  folder picker.
- **Frontend**: React 18 + TypeScript + Vite. Terminal is xterm.js 6 + `@xterm/addon-webgl`. Service shapes are
  hand-owned in `frontend/src/lib/api-types.ts` (mirrors of the Go structs' JSON tags — keep in sync).
- **Build/tasks**: [Task](https://taskfile.dev) — see Commands.

## Commands

```bash
task dev      # dev mode: Vite HMR + backend; separate DB, port and Chromium profile
task build    # frontend build + static Go binary → bin/lich
task run      # build + run
task test     # go test ./... + frontend vitest
task package  # .deb, .rpm, .pkg.tar.zst into bin/ (needs nfpm)
```

Frontend in isolation: `cd frontend && pnpm run build` (runs `tsc` + `vite build`).

---

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Test coverage ≥ 80%.** Backend (Go) and frontend (React/TS). Without a test, the feature is not ready. Run the
   suite before marking any task done; if it fails, fix it first. OS/framework boundaries (the PTY, the Chromium
   launcher/zenity subprocesses, WebSocket wiring, the `main` bootstrap, xterm.js internals) are a documented
   exception: cover the pure logic and leave the boundary itself alone (invariant 2 owns why).

2. **Tests answer to the contract, never the other way round.** A test earns its keep by failing when the product
   breaks. Never weaken, skip, delete or rewrite one to buy a green run or a coverage number, and never mock the
   framework to inflate it — a suite bought that way lies, and a lying suite is worse than a red one. A test may
   change for exactly two reasons: the contract changed, or the test asserts something the contract never promised
   (that is a broken test — name the real contract in the diff and say why the old assertion was wrong). "The
   assertion was in my way" is not one of them. When in doubt, change the product, not the test.

3. **A flake is a bug, and it has a root cause.** Never re-run CI until it goes green, never mark work done on a
   suite that passes "most of the time", and never dismiss a failure as "unrelated" without proving it. Reproduce it
   (`go test -count=200 -run TestX ./pkg/`), find the cause, fix that, and measure the failure rate before and after
   — a fix you cannot show as a rate change is a guess.

4. **Clean code.**
    - Small, focused functions (< 50 lines), one responsibility.
    - Cohesive files (200–400 lines typical, 800 max). Many small files > few large ones.
    - No deep nesting (> 4 levels) — use early returns.
    - Descriptive names; code explains itself, comments only for the *why*.
    - Errors handled explicitly, never silently swallowed.
    - No hardcoded magic values — use constants or config.
    - No secrets in source.

---

## Release Checklist

Releases are cut by pushing a `vX.Y.Z` tag. The `.github/workflows/release.yml` workflow runs the test suites, builds
the frontend, then `task package` produces the Linux artifacts (`.deb`, `.rpm`, Arch `.pkg.tar.zst`, plus the raw
static binary) and publishes a GitHub Release with a `checksums.txt`, taking the release notes from the
matching `CHANGELOG.md` section. The binary is pure Go — the workflow needs no C toolchain; frontend build comes
first because the backend `go:embed`s `frontend/dist`.

The package version comes from the git tag: the Taskfile computes `VERSION` via `git describe` (env `VERSION`
overrides) and injects it into `build/linux/nfpm/nfpm.yaml`.

Before tagging:

- [ ] Backend: `go test -race ./...` passes, `go vet ./...` clean, `gofmt` applied.
- [ ] Frontend: `cd frontend && pnpm build` (tsc + vite) and `pnpm test` pass.
- [ ] Move `CHANGELOG.md`'s `[Unreleased]` entries under a new `vX.Y.Z` heading and refresh the compare links.
- [ ] Optional local dry run: `task package` (writes the artifacts to `bin/`).
- [ ] Tag `vX.Y.Z` and push — the workflow builds, packages, and publishes the Release.

## Known Ceilings

Deliberate limits and shortcuts, with the upgrade path when it matters:

- **git status is polled every ~3 s** — one shared poller per repository path
  (`frontend/src/lib/git-status-store.ts`), no filesystem watch. Unchanged status short-circuits (same reference, zero
  re-renders). The lich plugin's `session-touched` hook nudges an immediate refresh after Claude edits files
  (`refreshGitStatus` in `frontend/src/lib/useGitStatus.ts`, driven from `docs/hooks/session-touched.md`), but the poll
  stays the baseline so it works without the plugin. Move to an fs watcher if the poll ever costs too much.
- **Persistence is hybrid**: UI preferences live in the page's `localStorage` (`lich.*` keys) — which physically lives
  in the Chromium profile and is keyed to the page origin, which is why the listener port is pinned (47821,
  `LICH_LISTEN_PORT` overrides; distinct from `LICH_PORT`, the per-session hook variable). The workspace lives in
  SQLite (`<config-dir>/lich/lich.db`). Closing a session does not delete its row (close ≠ delete). Card and tab order
  is a `position` column written whole on every drag and read back as `ORDER BY position, rowid`.
- **Hidden sessions are serialized and destroyed** (waveterm model, frontend edition): PTY output queues in a 2MB
  replay buffer (`frontend/src/lib/replay-buffer.ts`); show rebuilds from snapshot + tail; queue overflow drops the
  snapshot and starts clean from the tail (circular-buffer artifact contract). The replay state lives in the page —
  a full page reload loses hidden-session scrollback; a backend-kept tail (waveterm's filestore) is the upgrade path.
- **Terminal I/O rides the loopback WebSocket** — token auth, binary frames multiplexing all sessions
  (`internal/terminal/transport.go` ↔ `frontend/src/lib/term-transport.ts`). When the socket is down, output falls
  back to the `/events` channel and input to the RPC — slower, never broken.
- **No single-instance lock**: two lich processes race the pinned listener port; the second fails with a clear error.
  Add a lock + focus-existing-window protocol if it ever matters.
- **Reordering (cards, tabs) rides dnd-kit's pointer sensors.** `PointerSensor` needs its
  `activationConstraint.distance` (`frontend/src/lib/use-sortable-list.ts`) or the sensor claims the press and plain
  clicks stop selecting a session. dnd-kit over the HTML5 DnD API because it also gives the keyboard path and never
  mutates DOM order mid-drag.
- **No AppImage.** It cannot express dependencies, and a "portable" artifact that still needs a system Chromium
  betrays the format; bundling Chromium is a non-starter (sandbox needs SUID/user-namespaces — blocked on FUSE
  mounts and by Ubuntu 24.04's AppArmor — plus ~200MB and owning Chromium security patches). Install formats are
  `.deb`/`.rpm`/`.pkg.tar.zst` (deps as Recommends; pacman has no Recommends, only optdepends) plus `install.sh`.
  If bundling ever matters, that's option 2 (CEF) of `docs/chromium-shell.md`, not an AppImage.
