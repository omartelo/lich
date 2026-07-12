# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/omartelo/lich/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/omartelo/lich/releases/tag/v0.1.0
