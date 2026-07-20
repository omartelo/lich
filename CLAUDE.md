# lich

## What it is

`lich` is a **personal harness** — a desktop app whose Go backend serves an embedded React frontend to a system
Chromium window in `--app` mode (no Electron, no webview toolkit; see `docs/chromium-shell.md` for the full decision
record). It is shaped by the author's experience with and taste for other harnesses on the market; it is not a generic
product, it is a bespoke tool. Linux first; an experimental Windows build ships alongside it (see Known Ceilings).

## Stack

- **Backend**: Go 1.25, pure Go (CGO_ENABLED=0, fully static binary). Services exposed to the frontend over loopback
  HTTP RPC (`internal/rpc`) + WebSockets (terminal I/O in `internal/terminal/transport.go`, app events in
  `internal/events`), all on one token-authenticated listener. OS-specific code is selected by build tags behind
  small seams, never by runtime checks — the PTY is the model (`internal/terminal`'s `startPTY`: creack/pty on Unix,
  ConPTY on Windows, where npm's `claude.cmd` shim runs through `cmd.exe /c`).
- **Shell**: system Chromium-family browser launched in `--app` mode (`internal/chromium`), persistent profile under
  the user config dir (`~/.config/lich/chromium-profile`; `%AppData%\lich` on Windows; `~/Library/Application
  Support/lich` on macOS — all via `os.UserConfigDir`). Window closed = app exit. Runtime needs: a Chromium-family
  browser — chromium/chrome/brave on PATH on Linux, plus zenity for the folder picker; chrome/edge/brave via their
  conventional install paths on Windows (picker is native win32, and Edge being everywhere guarantees a window);
  Chrome/Chromium/Edge/Brave in their `.app` bundles on macOS (picker is osascript, built in).
- **Frontend**: React 18 + TypeScript + Vite. Terminal is xterm.js 6 + `@xterm/addon-webgl`. Service shapes are
  hand-owned in `frontend/src/lib/api-types.ts` (mirrors of the Go structs' JSON tags — keep in sync).
- **Build/tasks**: [Task](https://taskfile.dev) — see Commands.

## Commands

```bash
task dev              # dev mode: Vite HMR + backend; separate DB, port and Chromium profile
task build            # frontend build + static Go binary → bin/lich
task build:windows    # cross-compiles bin/lich.exe (experimental)
task run              # build + run
task test             # go test ./... + frontend vitest
task package          # .deb, .rpm, .pkg.tar.zst into bin/ (needs nfpm)
task package:windows  # bin/lich-setup.exe installer (needs Inno Setup 6 — CI/Windows)
```

Frontend in isolation: `cd frontend && pnpm run build` (runs `tsc` + `vite build`).

## Local Gate (before every commit / PR)

Run the whole check locally before pushing a commit or opening a PR — never lean
on CI to find what a local run catches in seconds:

- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `go test ./...` (backend) and `cd frontend && pnpm test` (frontend) green —
  or `task test` for both at once.
- `cd frontend && pnpm build` succeeds (tsc typecheck + vite).

CI mirrors exactly these: `ci.yml` runs them on every PR and push to `main`,
`release.yml` on a tag. Both render pass/fail counts and coverage into the
Actions job summary (`.github/scripts/*-test-summary.sh`), so the numbers are
visible per run rather than buried in the log. A red test fails the job — the
summary is transparency, never a substitute for the gate.

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

Releases are cut by pushing a `vX.Y.Z` tag. The `.github/workflows/release.yml` workflow fans out into two parallel
jobs — `linux` (frontend tests, backend tests, then `task package`: `.deb`, `.rpm`, Arch `.pkg.tar.zst`, plus the raw
static binary) and `windows` (`task package:windows`: exe + Inno Setup installer, then the backend suite on a real
Windows runner) — and a
`release` job fans in, checksums every asset into one `checksums.txt` and publishes the GitHub Release, taking the
notes from the matching `CHANGELOG.md` section. The binary is pure Go — no C toolchain anywhere; each job builds the
frontend first because the backend `go:embed`s `frontend/dist`. A `workflow_dispatch` run from any branch exercises
the whole pipeline (artifacts included) without publishing — use it to validate CI changes before a tag.

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

- **Session cwd is polled every ~2 s** from the PTY child
  (`internal/terminal/cwd.go`), emitted as `session-cwd` only on change and kept
  frontend-side in `session-cwd-store.ts` (never persisted; every PTY spawn
  re-reports its start directory). Per-platform reads behind the usual build-tag
  seam: `/proc/<pid>/cwd` on Linux, `proc_pidinfo(PROC_PIDVNODEPATHINFO)` on
  macOS (bound from libSystem x/sys-style — cgo_import_dynamic + asm trampoline,
  still pure Go), and a PEB walk on Windows (`NtQueryInformationProcess` +
  `ReadProcessMemory`; assumes the child matches our architecture — a 32-bit
  child degrades to the start path). Any failed read degrades the card to the
  directory the session started in. Reading the direct child misses a nested
  shell's `cd`, which is fine: the card tracks the session's shell, not
  arbitrary descendants.
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
  snapshot and starts clean from the tail (circular-buffer artifact contract). That page-side buffer only bridges
  hide→show within one page load; a **backend replay tail** (`internal/terminal/replay.go`, a 2MB in-memory ring per
  running session, matching the frontend cap) survives a full page reload, when the PTY lives on but the page-side
  scrollback is gone. On mount `TerminalView` fetches it via `terminal.Replay` and writes it before wiring the live
  listeners — so the tail lands ahead of any live frame, and output produced during that round-trip is dropped
  (a small seam gap, not a dup or a reorder — `term-transport` drops frames for an unlistened session). The ring is
  in memory per session; waveterm's disk filestore is the upgrade path if session count or size ever makes that cost
  matter.
- **Terminal I/O rides the loopback WebSocket** — token auth, binary frames multiplexing all sessions
  (`internal/terminal/transport.go` ↔ `frontend/src/lib/term-transport.ts`). When the socket is down, output falls
  back to the `/events` channel and input to the RPC — slower, never broken.
- **Single instance via the pinned port; focus is best-effort.** The pinned listener bind *is* the lock — only one
  process holds it. A second launch that cannot bind reads `runtime.json` (`{pid,port,token}`) and pings the recorded
  instance's token-gated `/ping` (`internal/singleton`): a live lich on the same port means a duplicate launch, so it
  focuses that window and exits 0 instead of erroring; anything else (a stray process on the port, a restart successor
  that never got the port back) still logs and exits 1. Focus hands the running instance's URL to Chromium against the
  shared profile, letting Chromium's profile-lock IPC forward to the running browser — the only *portable* raise, since
  an external process cannot raise a window under Wayland. It is untested against a real window and may open a second
  app window on some Chromium builds (`focusRunning` in `main.go`); a per-platform window raise is the upgrade path if
  it matters.
- **Self-update checks at startup, then hourly** (`internal/appupdate` + `frontend/.../AppUpdateGate.tsx`) — a
  `setInterval` poll in the gate, so a long-running session eventually notices a release mid-run; a session-scoped ref
  keeps the poll from stacking a second toast for a release already shown (a genuinely newer one still toasts). Hourly
  respects the unauthenticated GitHub API's 60-req/hour limit. Self-*apply* (download + checksum + in-place swap via
  `minio/selfupdate`) is Windows/macOS only, where lich owns its binary; on Linux the binary is package-manager owned,
  so the flow pastes the `install.sh` one-liner into a terminal and relaunches via `/restart` instead. The restart
  (`internal/restart`) spawns a detached successor that retries the pinned port (`LICH_RESTART_WAIT`) while the old
  process closes its window and exits — so the window blinks briefly, and if the successor cannot bind within ~10s it
  gives up and the user reopens by hand. Auto-restart is Unix-only (setsid); a Windows/macOS self-apply asks for a
  manual restart.
- **Reordering (cards, tabs) rides dnd-kit's pointer sensors.** `PointerSensor` needs its
  `activationConstraint.distance` (`frontend/src/lib/use-sortable-list.ts`) or the sensor claims the press and plain
  clicks stop selecting a session. dnd-kit over the HTML5 DnD API because it also gives the keyboard path and never
  mutates DOM order mid-drag.
- **No AppImage.** It cannot express dependencies, and a "portable" artifact that still needs a system Chromium
  betrays the format; bundling Chromium is a non-starter (sandbox needs SUID/user-namespaces — blocked on FUSE
  mounts and by Ubuntu 24.04's AppArmor — plus ~200MB and owning Chromium security patches). Install formats are
  `.deb`/`.rpm`/`.pkg.tar.zst` (deps as Recommends; pacman has no Recommends, only optdepends) plus `install.sh`.
  If bundling ever matters, that's option 2 (CEF) of `docs/chromium-shell.md`, not an AppImage.
- **Windows is experimental.** What holds it up: the backend suite runs on a Windows CI runner every release, the
  window/terminal path was smoke-tested by hand, and releases ship an Inno Setup installer
  (`build/windows/lich.iss` — per-user, Start Menu, Installed-apps entry, uninstaller; the exe icon is
  `rsrc_windows_amd64.syso` at the repo root, regenerated from `build/appicon.png` via `rsrc` if the icon ever
  changes). What's missing: no code signing (SmartScreen warns until download reputation accrues — a paid
  certificate is the fix if it ever matters), no winget manifest (the follow-up once a release with the installer
  is out), no Windows PTY tests (`terminal_test.go` is `!windows`; conpty-backed spawn tests are the gap to close
  before the tag can narrow), and the shell session is `COMSPEC`/cmd.exe — no PowerShell preference yet. The build
  is GUI subsystem (`-H=windowsgui`): no console rides along, stdout/stderr go nowhere, and
  `%AppData%\lich\lich.log` is the diagnostic surface — "double-clicked and nothing happened" means read the log.
- **macOS is experimental.** What holds it up: the codebase is pure Go and cross-compiles for darwin, and because
  macOS is Unix the terminal (creack/pty), shell and folder picker (osascript, via ncruces/zenity) already run
  through the shared `!windows` seams — the only macOS-specific code is `candidates_darwin.go`, which finds
  Chrome/Chromium/Edge/Brave in their `.app` bundles under `/Applications` and `~/Applications` (macOS installs
  never land on PATH). The backend suite — the real PTY tests included — runs on a `macos-latest` (Apple Silicon)
  runner every release, and both-arch raw binaries (`darwin-arm64`, `darwin-amd64`) ship as assets. What's missing:
  no code signing or notarization (Gatekeeper quarantines an unsigned binary — right-click-Open or
  `xattr -d com.apple.quarantine com.apple.provenance` until an Apple Developer cert + notarization ship), no
  `.dmg`/Homebrew packaging (raw binaries only, like the portable Windows exe), and the window path has not been
  hand-smoke-tested on real hardware yet.
