# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- An open pull request now badges the session card too, not just the footer. The
  same `PR #N` chip the footer shows for the active session appears on every card
  whose branch has an open PR, so one is visible per session without selecting
  the card first; clicking it opens the PR in the browser. Reuses the footer's PR
  lookup, is hidden when `gh` is absent or unauthenticated, and clears when the
  PR merges or closes.
- Windows releases now ship an installer (`lich-*-windows-amd64-setup.exe`,
  Inno Setup): per-user install under `%LocalAppData%\Programs\lich` with no
  admin prompt, Start Menu entry, a proper "Installed apps" registration with
  an uninstaller, and the lich icon on the executable. The bare portable exe
  keeps shipping alongside it.
- lich keeps a persistent log: `<config-dir>/lich/lich.log` (`lich-dev.log`
  under `task dev`), structured records with source file:line, rotated at 5MB
  with one previous generation kept. `LICH_LOG_LEVEL` (`debug`/`warn`/`error`)
  tunes verbosity. Every RPC failure is recorded with its method name, and the
  session token never reaches the log. This is the audit trail the future
  console-less Windows build will rely on.
- Experimental Windows build (`lich.exe`, `task build:windows`). Terminal
  sessions run under ConPTY, the window opens in Chrome or Edge (found via
  their conventional install paths), shell cards fall back to `COMSPEC`, and
  npm's `claude.cmd` shim is spawned through `cmd.exe /c`. Releases now ship a
  `windows-amd64.exe` asset built — and backend-tested — on a Windows runner
  in parallel with the Linux packages.
- Experimental macOS build (`lich-*-darwin-arm64`, `lich-*-darwin-amd64`,
  `task build:mac`). macOS is Unix, so the terminal (creack/pty), the shell and
  the native folder picker already work through the shared seams; only the
  Chromium launcher gained a macOS list, finding Chrome/Chromium/Edge/Brave in
  their `.app` bundles under `/Applications` and `~/Applications` (they never
  land on PATH). Releases now ship both-arch darwin binaries, built and
  backend-tested — the PTY included, on a real macOS runner — alongside the
  Linux and Windows jobs. Unsigned: Gatekeeper quarantines them until
  notarization ships (see the README/Known Ceilings).

### Changed

- The Windows binary is now a GUI-subsystem build: launching `lich.exe` no
  longer drags a console window along, and closing that console can no longer
  kill the app by accident. Logs live in `%AppData%\lich\lich.log` — the
  console mirror became best-effort so a missing stderr never poisons the
  file half of the log.
- Scrollbars are now discreet across the app. The heavy native Chromium
  scrollbar is replaced by a thin translucent thumb (diff, settings, sidebar,
  tabs) via a single global `::-webkit-scrollbar` rule; the terminal keeps its
  existing 6px overlay.

### Fixed

- A single-line selection in the diff review panel built a file reference with a
  redundant end (`path:19-19`). It now collapses to `path:19`, keeping the
  `start-end` range form only when the selection spans more than one line — for
  both the injected PTY reference and the context-menu label.

## [0.5.0] - 2026-07-15

### Added

- Reopening a session card that ran Claude Code before the last restart now asks
  whether to resume that conversation. Accepting spawns `claude --resume` on the
  session id the `SessionStart` hook recorded, so the card picks up where it
  stopped; declining starts an empty session as before. The prompt is asked once
  per card, the first time it is opened after a restart, and never for a shell
  card or one created in this run.
- A project tab badges what its sessions are doing while you work elsewhere: a
  bell when one is blocked waiting on you, a spinner while one is running, a
  check when a turn finished. The active tab never badges — its cards already
  say the same thing, per session. The check clears once the project has been on
  screen; the bell and the spinner stay while they are true, so a tab you leave
  mid-run keeps saying so.

### Fixed

- A session card kept its status indicator (spinner, check, bell) when its
  project was not the one on screen. Switching projects mid-run unmounted the
  card and dropped the state along with it, so coming back showed no spinner for
  a session Claude was still working on; the state now lives in a store that
  outlives the card. A session that starts needing your input while in the
  background also shows its bell once the toast routes you to it.

## [0.4.0] - 2026-07-15

### Added

- Drag a session card or a project tab to reorder it. The list rearranges live
  under the cursor and the new order is persisted, so it survives a restart;
  releasing outside the list (or pressing Escape) leaves the order untouched.
  Reordering also works from the keyboard: focus a card or tab, then Space and
  the arrow keys.
- `install.sh` — one-liner install (`curl ... | sh`) that detects the distro,
  downloads the matching package from the latest release, verifies its
  checksum and installs it through the native package manager, then checks
  the runtime dependencies (Chromium-family browser, zenity) are present.

### Changed (BREAKING — new shell)

- lich now opens in the system Chromium's `--app` mode instead of the
  WebKitGTK webview, eliminating the compositor paint jank for good (decision
  record: `docs/chromium-shell.md`). The Wails toolkit, the bundled WebKitGTK
  and the `GDK_BACKEND=x11` workaround are gone; the binary is pure static Go.
  New runtime requirements: a Chromium-family browser on PATH and zenity for
  the folder picker. UI preferences (theme, font, hotkeys) reset once — they
  now live in the Chromium profile; the workspace (projects/sessions, SQLite)
  carries over untouched.
- The terminal is now xterm.js with the WebGL (GPU) renderer, replacing the
  patched ghostty-web canvas pipeline — noticeably smoother TUI scrolling and
  streaming, and correct Shift+Tab/Alt-chord/mouse handling without patches.
  Hidden sessions no longer keep a terminal at all: their state is serialized
  and replayed on return, cutting memory with many open sessions.

### Changed

- Diff counters (+added/−deleted) now use one palette everywhere: green for
  additions, red for deletions. Session cards and the footer previously showed
  them in blue/pink while the review panel used green/red.
- Backend services are now reachable over the loopback listener (HTTP RPC +
  event socket); the frontend no longer depends on the Wails binding bridge.
- Hidden sessions no longer hold a canvas backing store (several MB each at
  window size). The bitmap is released when a session leaves the screen and
  transparently reallocated on the next paint when it returns, cutting webview
  memory with many open sessions.
- Spawned shells keep a user-set `WEBKIT_DISABLE_*` variable — it was stripped
  as packaging leakage back when lich's own AppImage set it; nothing does
  since the WebKitGTK shell was removed.

### Removed

- The AppImage artifact. It cannot declare dependencies, and lich needs a
  system Chromium and zenity at runtime either way — a "portable" AppImage
  that isn't self-contained betrays the format. Install via `install.sh`
  (detects the distro, verifies checksums, installs the native package) or
  grab the `.deb`/`.rpm`/`.pkg.tar.zst` directly; the bare static binary
  also ships with every release.

## [0.3.0] - 2026-07-14

### Added

- Claude Code plugin integration. lich pairs with a companion Claude Code
  plugin ([`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin)),
  installed and updated from within the app: a one-click install modal when it
  is missing, and an actionable toast when a newer plugin release ships. The
  plugin reports session activity back over the existing loopback transport;
  every contract is documented in `docs/hooks/`, which lich owns as the
  canonical source and the plugin references.
- Session cards reflect Claude Code state live — a spinner while Claude is
  producing output, a check when the turn ends, and a bell when Claude is
  blocked on you (a permission prompt or an idle input request). The bell also
  raises an actionable toast that routes to the waiting session, reachable even
  when it lives in a background project, and skipped for the session already on
  screen. A stale indicator clears when the session ends or is `/clear`ed.
- Sessions auto-name from Claude's own title. When Claude generates its session
  summary (the `ai-title` shown in `claude --resume`), lich adopts it as the
  card label — unless you have renamed the session, which always wins.
- A session's git badge refreshes the moment Claude edits files, ahead of the
  ~3s poll, so the diff counts and branch stay current without the lag. The
  poll stays the baseline, so the badge works unchanged without the plugin.
- An open pull request for the active session's branch surfaces as a clickable
  badge in the footer — `PR #N` with a pull-request icon — that opens the PR in
  the OS browser. It resolves the PR through the `gh` CLI, shows only while the
  PR is open (a merged or closed one clears it), and re-checks on window focus
  so a merge done in the browser drops the badge on return. Hidden when `gh` is
  absent or unauthenticated.

### Changed

- The footer's diff toggle now renders as a bordered muted chip, with a diff
  icon in its zero-change state, matching the new PR badge's look.
- The Linux Arch package no longer hardcodes `pkgrel` in `nfpm.yaml`. nfpm
  defaults it to `1`, so the produced `.pkg.tar.zst` version is unchanged
  (`X.Y.Z-1`) — the field just isn't pinned in the repo anymore. `pkgrel` is
  mandatory in the Arch package format and has no source in the git tag, so it
  stays `1` rather than being derived.

## [0.2.0] - 2026-07-13

### Added

- Settings gained a "Project" group with a per-project Claude Code binary
  override. The backend already resolved project → global → `$PATH`; the
  override just had no UI.
- Terminal URLs (OSC 8 hyperlinks and detected URLs) now hover-underline and
  open in the OS browser on Ctrl/Cmd-click. ghostty-web ships link detection
  but registers no provider by default, and its `window.open` is trapped by
  the WebKitGTK webview; lich registers both providers and routes clicks
  through Wails' `Browser.OpenURL` to the desktop default.

### Fixed

- Mouse wheel now scrolls instead of sending arrow keys. ghostty-web reports
  no mouse events, so its alternate-screen emulation turned each wheel tick
  into an arrow key — which Claude Code flagged as "arrow keys · use PgUp/PgDn
  to scroll". The wheel now forwards a real SGR report to apps with mouse
  tracking (they scroll by their own line increment), falls back to PgUp/PgDn
  in the alternate screen otherwise, and scrolls ghostty's own scrollback
  everywhere else.
- Stray editable nodes no longer accumulate in the terminal container. On the
  forced X11 backend, middle-click primary-selection paste and drag-drop
  inserted editable nodes past ghostty's `beforeinput` guard, pushing the
  in-flow canvas down and leaving selectable text behind; a `MutationObserver`
  now removes any node other than the canvas and textarea ghostty owns.
- Terminal sessions no longer inherit the AppImage's runtime environment.
  Beyond the vars stripped in #3, `childEnv` now drops the AppImageLauncher
  vars and the `WEBKIT_DISABLE_*` pair, and scrubs mount paths out of
  `LD_LIBRARY_PATH`, `PATH`, `XDG_DATA_DIRS`, `GDK_PIXBUF_MODULE_FILE` (and any
  future path list) — the bundled Ubuntu libs broke linkers and GTK apps
  launched from a lich terminal. User-set entries survive; outside an AppImage
  the environment passes through untouched.
- The `GDK_BACKEND=x11` forced at startup no longer leaks into spawned
  sessions: the session environment is snapshotted before the GTK tweak.
- `task dev` now opens alongside an installed lich: dev instances register a
  distinct GTK application ID (`lichdev`), so GTK single-instance no longer
  swallows the dev window when the AppImage is running.

## [0.1.1] - 2026-07-12

### Fixed

- The AppImage aborted on startup on any distro that is not Debian/Ubuntu
  (`Failed to spawn child process ".../webkitgtk-6.0/WebKitNetworkProcess"`).
  The bundled WebKitGTK hardcodes its helper paths at compile time, so the
  packaging now binary-patches `libwebkit*` to resolve them inside the AppDir
  (the same relocation tauri's bundler applies), marks the bundled
  `WebKit*Process` helpers executable (wails3 copies them without the exec
  bit), and disables the webkit sandbox, which would otherwise require the
  host's `bwrap` at another baked path. See
  `build/linux/appimage/fix-appimage.sh`.

### Added

- `LICH_DEV` environment variable: when set, the app uses a separate SQLite
  database (`lich-dev.db` instead of `lich.db`), keeping development work away
  from the real workspace. `task dev` sets it automatically.

## [0.1.0] - 2026-07-12

### Changed

- Terminal rendering is ~4× faster under heavy TUI load (nvim scroll worst-case
  main-thread stall down from ~200-250ms to ~40-70ms; idle paint cost down ~8×).
  Four changes: PTY output is now coalesced on the Go side for visible sessions
  too (8ms batches; hidden stays at 250ms), blank cells skip `fillText`
  entirely, cell backgrounds are painted as one `fillRect` per same-color run
  instead of per cell, and plain glyphs are cached on offscreen sprites and
  blitted with `drawImage` instead of re-rasterized with `fillText` every
  frame. The remaining ceiling is ghostty-web's per-row WASM cell
  materialization (`getLine`), which is only fixable upstream.

### Added

- Git diff review panel: the footer's diff counters toggle a resizable split at
  the terminal's right showing the active session's uncommitted changes, one
  collapsible card per file with syntax highlighting, line numbers, and hunk
  separators (CodeMirror 6). Selecting lines and right-clicking injects
  `@path` or `path:start-end` references into the session's PTY; per-file
  buttons add the file as context or discard its changes after confirmation.
  A full-screen mode overlays the terminal area.
- PTY-backed terminal harness with multiple sessions per project.
- Multi-project workspace: open projects through the OS picker and switch between
  them via a Discord-style rail with tabs.
- Session cards showing the working directory, git branch, a diff badge, and an
  untracked-line count.
- Appearance settings: System/Light/Dark theme, UI zoom, and a separate terminal
  theme.
- Configurable hotkeys, including terminal-aware zoom.
- Warp-style footer bar with git status and file attach.
- Git worktree sessions: create a worktree from a local or remote base branch
  (fetched and tracked) with an optional custom or auto-generated name, resume
  an existing worktree, and open Claude Code directly in its checkout. Closing
  the session asks whether to keep or remove the worktree — removing one with
  uncommitted changes asks a second confirmation before forcing — and session
  cards and the footer follow the worktree's path, branch, and diff.
- Right-click context menu to rename or close a session.
- Bundled FiraCode Nerd Font.
- Configurable Claude Code binary path in settings.
- Toast feedback when copying from the terminal.
- New-session dropdown on the sidebar "+": spawn a Claude Code session or a
  plain shell terminal; the session type persists and restores with the
  workspace.
- Workspace persisted in SQLite; UI preferences in `localStorage`.

### Changed

- Renamed the project from `skipo` to `lich`: Go module
  `github.com/omartelo/lich`, app and binary name, data directory
  `<data-dir>/lich/lich.db`, `lich.*` `localStorage` keys, and every platform
  build asset.
- Set release metadata in `build/config.yml` (product `lich`, identifier
  `dev.lich.app`, version `0.1.0`).
- Renamed the `internals` package to `internal`.
- Translated `CLAUDE.md` to English.
- Home paths render with a `~` prefix and an overflow fade on cards.
- Switched the base color palette from zinc to neutral.

### Fixed

- Terminal now fills the container edge to edge — replaced ghostty-web's
  FitAddon, which reserved a fixed 15px scrollbar gutter and left a band on the
  right.
- Hid the native caret over the terminal canvas.
- Synthesized block-element glyphs in the terminal renderer.
- Derived cell height from the font bounding box.
- Debounced terminal refit to keep window drags fluid.
- Focus the previous tab when closing the active project.
- Shift+Tab now reaches terminal apps as backtab (`ESC [ Z`) and Alt chords get
  their ESC prefix — ghostty-web 0.4.0 drops both, and WebKitGTK reports
  Shift+Tab as the `ISO_Left_Tab` keysym.
- Long worktree paths wrap instead of overflowing the close dialogs.

### Performance

- Spawn session PTYs lazily on first view.
- Lowered the git-status poll interval to 3 s.
- Paused the ~60fps render loop of hidden terminals; only the visible terminal
  paints (state keeps updating, so switching back repaints instantly).
- Coalesced PTY output of hidden sessions in the backend to one event per 250 ms,
  flushed immediately when the session is shown.
- Skipped the resize-driven refit for hidden terminals; they refit once on show.
- Shared one git-status poller per repository path with equality bailout: with
  20+ session cards the idle burst of ~44 IPC calls and ~88 git subprocesses
  every 3 s collapses to one fetch per path, and unchanged status no longer
  re-renders anything.
- Removed the per-cell defensive copy in ghostty-web's `getLine` (pool-backed
  row references), gated rendering while scrolled with nothing dirty, and
  memoized scrollback lines — reading scrollback now costs ~0 paint, and heavy
  TUI throughput renders at ~40fps instead of ~25.
- Terminal I/O now flows over a local binary WebSocket (random loopback port,
  token-authenticated) instead of one Wails HTTP call per keystroke and one
  `evaluate_javascript` per output chunk; falls back to the Wails paths
  automatically if the socket drops.
- Forced `GDK_BACKEND=x11` on Linux (only when unset): WebKitGTK under Wayland
  fractional scaling rendered every damage frame at 2x and downsampled on the
  CPU, costing ~40ms per frame in a full-size window. Under Xwayland typing is
  stall-free at full frame rate.

[Unreleased]: https://github.com/omartelo/lich/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/omartelo/lich/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/omartelo/lich/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/omartelo/lich/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/omartelo/lich/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/omartelo/lich/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/omartelo/lich/releases/tag/v0.1.0
