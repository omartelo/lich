<div align="center">
  <h1>lich</h1>
  <p><strong>A personal harness for coding with AI agents.</strong></p>
  <p>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-Chromium%20--app-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
  </p>
</div>

## About

`lich` is a **personal harness** — a desktop app that wraps a terminal-first
workspace around AI coding agents like
[Claude Code](https://www.anthropic.com/claude-code). Open several projects, run
a session (or many) per project, and keep an eye on git state without leaving the
window.

It is shaped by the author's taste for other harnesses — Warp and friends — and
is deliberately bespoke: not a generic product, a tool built for one workflow.
It's public because there's no reason to hide it, not because it's a supported
release.

> **Platform** — Linux first; experimental macOS and Windows builds ship
> alongside it. lich is a single static Go binary that opens its UI in your
> system's Chromium-family browser in `--app` mode (no Electron, no bundled
> webview — decision record: `docs/chromium-shell.md`). At runtime it needs a
> Chromium-family browser: on **Linux**, chromium/google-chrome/brave on PATH
> plus `zenity` for the folder picker; on **macOS**, Chrome/Chromium/Edge/Brave
> in `/Applications` (the picker is built in); on **Windows**, Chrome/Edge/Brave
> via their usual install paths. The macOS and Windows binaries are unsigned —
> Gatekeeper and SmartScreen warn until notarization/signing ship.

## Features

- **Terminal-first, PTY-backed sessions** — real shells, multiple per project,
  rendered by xterm.js on the GPU (WebGL).
- **Multi-project workspace** — open projects through the OS picker and switch
  between them via a Discord-style rail with tabs.
- **Live git at a glance** — session cards show the working directory, current
  branch, a diff badge, and untracked-line count; a Warp-style footer bar carries
  git status and file attach.
- **Claude Code integration** — point lich at your `claude` binary and drive it
  from any session.

## Install

Detects your distro, verifies the checksum and installs the native package and
its dependencies through your package manager:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

**Manual** — per-distro packages, runtime dependencies and the static binary:
[INSTALL.md](INSTALL.md).

## Build from source

Prerequisites: **Go 1.25+**, **Node + pnpm**, **[Task](https://taskfile.dev)**.
No C toolchain, no system dev libraries — the binary is pure Go
(`CGO_ENABLED=0`).

```bash
task dev      # hot-reload dev mode (Vite on :9245)
task build    # production binary -> bin/lich
task run      # build + run
task test     # Go + frontend suites
```

Package a Linux release locally (needs
[nfpm](https://nfpm.goreleaser.com/)):

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

## Configuration

- **Claude Code binary** — set its path in Settings.
- **Appearance & hotkeys** — configured in Settings; UI preferences persist in
  `localStorage` under `lich.*` keys (inside lich's Chromium profile at
  `~/.config/lich/chromium-profile`).
- **Workspace** — projects and sessions persist in SQLite at
  `<config-dir>/lich/lich.db`. Closing a session does not delete it.

## Stack

Pure-Go backend (Go 1.25, `CGO_ENABLED=0`) serving an embedded React 18 /
TypeScript / Vite frontend to a system Chromium window over a token-authenticated
loopback listener (HTTP RPC + WebSockets). Terminal rendering is xterm.js with
the WebGL addon. See `docs/chromium-shell.md` for how it got this way.

## License

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich is free software: you can use, study, modify and redistribute it under
the terms of the GNU Affero General Public License v3. Any distributed or
network-served derivative must be released under the same license. Releases
up to and including v0.9.0 remain MIT-licensed.
