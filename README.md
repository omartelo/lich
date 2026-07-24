<div align="center">
  <img src="frontend/public/appicon.png" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><strong>A terminal-first harness for coding with AI agents.</strong></p>
  <p>
    Open your projects, run agents like Claude Code, Codex, opencode or Crush in
    real terminals, and keep git — worktrees and all — in view without leaving
    the window.
  </p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-Chromium%20--app-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
  </p>
</div>

## About

`lich` is a **personal harness** — a desktop app that wraps a terminal-first
workspace around AI coding agents. Open several projects, run a session (or
many) per project, drive an agent in each, and watch git state — branches,
diffs, worktrees — without ever leaving the window.

It ships as a single static Go binary that opens its UI in your system's
Chromium-family browser in `--app` mode — no Electron, no bundled webview
(decision record: [`docs/chromium-shell.md`](docs/chromium-shell.md)).

It is deliberately bespoke: shaped by the author's taste for other harnesses
(Warp and friends), built for one workflow rather than as a generic product.
It's public because there's no reason to hide it, not because it's a supported
release.

## Supported agents

Drive any of them — set the binary path in Settings and pick which one new
sessions default to:

- **[Claude Code](https://www.anthropic.com/claude-code)** — Anthropic
- **[Codex](https://github.com/openai/codex)** — OpenAI
- **[opencode](https://github.com/sst/opencode)** — SST
- **[Crush](https://github.com/charmbracelet/crush)** — Charm

## Features

- **Bring your own agent.** The four above are all first-class. Point lich at
  each binary in Settings, choose a default, or pick per session.
- **Terminal-first sessions.** Real PTY-backed shells, several per project,
  rendered by xterm.js on the GPU (WebGL). Search the scrollback (`Ctrl+F`), and
  it survives a full page reload; terminal text size is its own setting.
- **Multi-project workspace.** Open projects through the OS folder picker and
  switch between them on a top tab bar. A project can sit with no session at all.
- **Git worktrees, built in.** Spin up a worktree from any base branch — search
  it, local or remote, even across dozens of branches — and lich seeds the new
  checkout with your gitignored `.env*` files and runs a per-project setup
  script before the agent starts. The sidebar groups sessions by the worktree
  they belong to, and a kept worktree's session is ready to resume later.
- **Review without leaving.** A CodeMirror diff dock shows the working changes;
  collapse or expand every file at once, and attach files to a session.
- **Live git at a glance.** Session cards carry the working directory, current
  branch, a diff badge and an untracked-line count; a Warp-style footer follows
  `cd` and surfaces git status.
- **Command palette.** `Ctrl`/`Cmd`+`K` to jump between sessions and projects.
- **Notifications.** A session waiting on your input raises a toast and a dot on
  the bell, collected in a titled dropdown.

## Install

One line — detects your distro, verifies the checksum, and installs the native
package and its dependencies through your package manager:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| Platform | Get it | Needs at runtime |
| --- | --- | --- |
| **Linux** | `install.sh` above, or AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | chromium / google-chrome / brave on `PATH`, plus `zenity` |
| **macOS** *(experimental)* | raw binary from [Releases](https://github.com/omartelo/lich/releases) | Chrome / Chromium / Edge / Brave in `/Applications` |
| **Windows** *(experimental)* | installer from [Releases](https://github.com/omartelo/lich/releases) | Chrome / Edge / Brave |

Manual per-distro packages and the static binary: [INSTALL.md](INSTALL.md). The
macOS and Windows binaries are unsigned — Gatekeeper and SmartScreen warn until
notarization/signing ship.

## Getting started

1. **Install** and launch `lich`.
2. **Open a project** — the `+` opens your OS folder picker; point it at a git
   repository.
3. **Point lich at your agent** — in Settings, set the binary path for Claude
   Code, Codex, opencode or Crush, and choose a default.
4. **Start a session** — *New Session* spawns a terminal running your agent in
   the project.
5. **Branch off a worktree** *(optional)* — create one from any base branch;
   lich seeds it and drops you into a fresh session.

## Configuration

- **Agents** — set each provider's binary path in Settings, and pick which one
  new sessions default to.
- **Worktrees** — a per-project setup script (Settings › Worktree) runs in a new
  worktree's terminal ahead of the agent; a `.worktreeinclude` file tunes which
  gitignored files get copied over.
- **Appearance & hotkeys** — configured in Settings; UI preferences persist in
  `localStorage` under `lich.*` keys (inside lich's Chromium profile at
  `~/.config/lich/chromium-profile`).
- **Workspace** — projects and sessions persist in SQLite at
  `<config-dir>/lich/lich.db`. Closing a session does not delete it.

## Privacy & updates

Everything runs on your machine. No account, no sign-in, no telemetry — the
backend is a token-authenticated loopback listener, and nothing leaves
`localhost` except the update check: a version ping to GitHub Releases at startup
and hourly. Updates apply in place on Windows/macOS and through the AUR on Arch.

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

## Stack

Pure-Go backend (Go 1.25, `CGO_ENABLED=0`) serving an embedded React 18 /
TypeScript / Vite frontend to a system Chromium window over a token-authenticated
loopback listener (HTTP RPC + WebSockets). Terminals are xterm.js with the WebGL
addon; the code/diff dock is CodeMirror 6. See
[`docs/chromium-shell.md`](docs/chromium-shell.md) for how it got this way.

## License

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich is free software: you can use, study, modify and redistribute it under
the terms of the GNU Affero General Public License v3. Any distributed or
network-served derivative must be released under the same license. Releases
up to and including v0.9.0 remain MIT-licensed.
