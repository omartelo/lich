# Decision: move the shell from WebKitGTK to Chromium

**Status: option 1 shipped in v0.4.0 (2026-07-15) and is still how an Intel
Mac opens the window. Option 2 shipped on Linux on 2026-09-05, then on
Windows and Apple Silicon: lich bundles its own Chromium (CEF, through
kurogane) and no browser is required — see the section at the end.**

## Why

lich's remaining paint jank is the WebKitGTK compositor itself, not our
rendering pipeline. Evidence, collected on the reference machine (Dell G15,
i7-13th, RTX 4050 + Intel iGPU via prime, Hyprland/Xwayland):

- `frontend/src/lib/gpu-probe.ts` proved the webview has **hardware WebGL2**
  (~39 Gpx/s fill vs ~1-2 Gpx/s llvmpipe-class). Renderer strings are masked
  by WebKit as "Apple GPU (Apple Inc.)" — anti-fingerprinting, ignore them.
- The xterm.js + WebGL proof of concept (`XtermTerminalView.tsx`, flag
  `lich.xtermPoc`) made e.g. nvim scrolling noticeably smoother — GPU text
  rendering works and helps.
- Jank persisted on WebKitGTK **2.52.4 (latest, Skia GPU paint)** under
  Xwayland: frame-pacing/compositing is WebKitGTK's ceiling, with no env knob
  left to try. Every fluid web terminal in the market (waveterm, VS Code,
  Hyper) sits on Chromium's compositor for a reason.

Electron is explicitly rejected as the way to get Chromium: we picked Wails to
avoid shipping Node, and that constraint stands. (For the record: waveterm's
"Go backend" is a sidecar child process of an Electron shell — it does not
avoid Electron.)

## Option 1 — system Chromium in `--app` mode (chosen)

The lorca pattern, hand-rolled (lorca itself is unmaintained; we need none of
its CDP surface):

- The Go binary serves the embedded frontend (`go:embed frontend/dist`) over
  loopback HTTP and launches
  `chromium --app=http://127.0.0.1:<port> --user-data-dir=<state-dir> --class=lich`.
- Window closed → WebSocket drops → Go shuts down. No CDP needed for v1.
- Still a single Go binary. **Zero Node, zero Electron, no new bundle weight.**
  New runtime requirement: a Chromium-family browser installed (fine for a
  personal harness on Arch; the launcher should probe `chromium`,
  `google-chrome`, `helium-browser`, `chromium-browser` and fail with a clear
  message).

Why lich is ~80% there already:

- Terminal I/O already rides a loopback WebSocket with token auth
  (`internal/terminal/transport.go` ↔ `frontend/src/lib/terminal/term-transport.ts`) —
  Chromium connects to it unchanged.
- The frontend is already embedded and static; serving it over HTTP replaces
  handing it to the webview.
- The lich-plugin hooks (`docs/hooks/`) already talk to the same transport —
  unaffected.

Migration progress:

1. **RPC-ify the Wails bindings** — DONE (phase 1): `internal/rpc` dispatcher
   on `POST /rpc/<service>.<Method>`, `internal/events` hub on `/events`,
   frontend facades in `lib/rpc.ts` / `lib/app-events.ts`. The Wails bridge
   remains only as the events fallback and the endpoint bootstrap.
2. **Chromium shell** — DONE (phase 2): `LICH_SHELL=chromium ./lich` serves
   the embedded frontend on the loopback listener (public mount; RPC/WS stay
   token-gated) and opens the system Chromium via `internal/chromium` on a
   persistent profile (`~/.config/lich/chromium-profile` — localStorage lives
   there, so the listener port is pinned to 47821, `LICH_LISTEN_PORT`
   overrides; NOT `LICH_PORT`, which is the per-session hook variable).
   Window closed = app exit. Extra flags: `lich -- --ozone-platform=wayland`.
   Folder/file pickers go through zenity (`project.ZenityPicker`); clipboard
   paste prefers `navigator.clipboard` with the Wails clipboard as fallback.
   Known gap: no single-instance lock yet — run one at a time.
3. **Terminal swap to xterm.js/WebGL** — DONE (phase 3): XtermTerminalView is
   the terminal in both shells (links via @xterm/addon-web-links +
   system.OpenExternal, copy-on-select toast, live font/theme). Hidden
   sessions are serialized (@xterm/addon-serialize) and destroyed; PTY output
   queues in a 2MB replay buffer (lib/replay-buffer.ts) and show rebuilds
   from snapshot + tail — the waveterm model, frontend edition. The ghostty
   WebKitGTK workarounds were not ported (they patched ghostty-web 0.4.0
   bugs); ghostty itself stays reachable via
   localStorage.setItem("lich.terminal", "ghostty") until phase 5.
4. **De-Wails the build** — DONE (phase 4, folded into 5): the binary is pure
   Go, fully static under `CGO_ENABLED=0` (modernc sqlite, creack/pty,
   coder/websocket, zenity are all CGO-free).
5. **Cleanup** — DONE (phase 5): the Wails path, the wailsapp dependency, the
   generated bindings, the ghostty-web terminal with its entire
   private-patching layer, the `GDK_BACKEND=x11` hack and the measurement
   spike are deleted. Chromium is the only shell; service shapes are
   hand-owned in `frontend/src/lib/api-types.ts`. The release pipeline ships
   the static binary: nfpm directly for .deb/.rpm/.pkg.tar.zst (chromium and
   zenity are *recommends*, never hard deps) and
   `build/linux/make-appimage.sh` wraps it in an AppImage with nothing
   bundled — `fix-appimage.sh` and the WebKitGTK payload are gone.
   Development runs through `task dev`: Vite HMR + backend, the window
   pointed at the dev server via `LICH_DEV_URL`, with a separate DB, listener
   port (47822) and Chromium profile so the daily-driver install stays
   untouched.

**The migration is complete.** This document remains as the decision record;
option 2 (embedded CEF via `energye/energy`) stays deferred with the same
trigger — the project growing distribution needs a system-browser dependency
can't serve.

What dies with WebKitGTK (all "Known Ceilings" entries): forced
`GDK_BACKEND=x11`, the sandbox-disabled AppImage, the contenteditable DOM
guard, middle-click-paste quirks — plus, if the xterm migration is confirmed
by the same spike, the entire ghostty-web private-patching layer
(`render-pause`, `glyph-atlas`, `row-paint`, `scrollback-perf`, `getline-pool`,
`block-glyphs`, `font-metrics`, the 0.4.0 pin).

Trade-offs accepted: the window belongs to Chromium (no native menus — unused
anyway; icon/class via flags), and the Chromium version tracks the system.

### Spike (shipped — `cmd/spike`)

One disposable binary: serves `frontend/dist` over loopback, opens
`spike.html` (a standalone xterm.js + WebGL terminal, no Wails/React) in the
system Chromium's `--app` mode, and bridges one PTY per WebSocket connection
— deliberately uncoalesced, one send per PTY read (the waveterm firehose).
A stats overlay reports `fps / stalls / worst` (rAF gaps, same vocabulary as
term-perf) every second.

```sh
cd frontend && pnpm build && cd ..
go run ./cmd/spike                             # picks chromium/chrome on PATH
go run ./cmd/spike -no-browser                 # just prints the URL
go run ./cmd/spike -- --ozone-platform=wayland # extra Chromium flags
```

Run the same scenarios as the WebKitGTK build (nvim scroll, Claude Code
streaming, `yes`), watch the overlay. Jank gone → commit to the migration
above. Files to delete when the decision lands: `cmd/spike/`,
`frontend/spike.html`, `frontend/src/spike/`, the `spike` input in
`frontend/vite.config.ts`.

## Option 2 — embedded CEF (shipped on Linux 2026-09-05, Windows next)

Chromium shipped with the app (CEF). No dependency on a system browser, the
Chromium version pinned per release, and a window that is lich's own: its
WM_CLASS / app_id, its title, its keyboard, no browser prompts, no system
extensions loading into it.

What was written when this was deferred still holds, and is now the price
paid: +100 MB per package download (~300 MB on disk), and a Rust toolchain
with CMake in CI. What changed is the route. `energye/energy` (Go bindings,
CGO) was never taken. The window is a **separate binary**, `shell/`, a Rust
crate on [kurogane](https://github.com/0x48piraj/kurogane) (cef-rs
underneath), and the Go binary launches it exactly the way it launches a
system browser — the same `internal/chromium.Args` argv, `--app=<url>`,
`--class`, `--user-data-dir`, the user's `--` switches. Nothing in the Go
side knows which one it got; `CGO_ENABLED=0` and the static binary stand.
The migration path really was "swap who provides the window": one new rung
in the resolution ladder, above the desktop's default and below the pin.

Measured on the reference machine (RTX 3050, Hyprland, Chromium 150 in CEF
against Helium 151 as the system browser): no perceptible difference, which
is the point — same engine, same GPU path, WebGL on ANGLE over the NVIDIA
driver in both. `seq 1 400000` into an xterm.js session paced at 84 rAF/s on
a 100 Hz display. Native Wayland, XWayland and X11 all open with the class
and title the Go side asks for.

kurogane needed two things it did not have — a WM_CLASS / app_id and a
title on the window it creates — so `shell/` builds against a fork carrying
that patch (`App::window_class`, `App::window_title`; cef-rs drops an owned
string on the way back into a CEF out-struct, so the fork allocates them
through CEF itself). The patch is upstream as
[kurogane#11](https://github.com/0x48piraj/kurogane/pull/11); the fork
(`omartelo/kurogane`) is the pin until it lands.

Windows ships the same window, flat beside `lich.exe` as `shell\` the way
CEF lays itself out there, inside the installer (`build/windows/lich.iss`)
and as a zip beside the portable exe. Two things the Linux window gets from
its WM_CLASS come from elsewhere on Windows: the executable carries lich's
icon and manifest as resources (`shell/build.rs`), and the process claims the
AppUserModelID the Start Menu shortcut declares, which is what makes the
taskbar group the running window under the pinned icon. The sandbox stays
off there, as kurogane runs it, so the binary is a plain exe rather than CEF's
`bootstrap.exe` loading a DLL; Linux is the one platform whose window runs
sandboxed — from a package, where the deb, rpm and AUR install Chromium's
setuid helper root-owned 4755 beside `lich-shell` for the desktops that deny
unprivileged user namespaces (`docs/ceilings.md`). Built and smoke-tested on
the CI runner only.

macOS ships the same window inside `Lich.app`, Apple Silicon only: the
release runner is arm64 and builds the window for itself, and the Intel
bundle keeps opening a system browser. `lich-shell` sits beside `lich` in
`Contents/MacOS`, because macOS reads a process's bundle off its executable's
path and only a process inside the bundle is `Lich.app` to the Dock, to
Cmd-Tab and to the menu bar; the framework goes to `Contents/Frameworks`,
where kurogane looks for it (upstream's `seal-of-approval/distribution`
branch, commit
[53d51ba](https://github.com/0x48piraj/kurogane/commit/53d51ba7d234161f8709109dcb8d6395f38c7bb4),
carried on the fork meanwhile; lich's #13 was closed as covered). CEF's
subprocesses run as the five helper apps beside the framework (`Lich
Helper` and the Renderer, GPU, Plugin and Alerts variants, each a copy of
the binary under an `LSUIElement` plist), the way CEF's own samples lay
them out, and kurogane points `browser_subprocess_path` at the base one (the same
upstream commit; lich's #15 was closed as covered). Both
halves were measured on the runner before they were written: re-executing
the window binary itself gave every subprocess a Dock tile of its own
(four tiles for one window, until
[kurogane#14](https://github.com/0x48piraj/kurogane/pull/14) kept the
`NSApplication` to the browser process) and, inside a bundle, never started
a renderer at all, since Chromium derives the renderer's executable from
the helper's path and finds nothing there. The bundle is ad-hoc signed
innermost first (ANGLE's dylibs, the framework, the helpers, the window,
the app), never notarized. The window keeps its cookie
store unencrypted (`CredentialStorage::Basic`): Chromium keys it through the
Keychain to one code identity, and an ad-hoc signature is a new identity per
build, so the alternative is a Keychain prompt on every start. Built and
smoke-tested on the CI runner only, like Windows.
