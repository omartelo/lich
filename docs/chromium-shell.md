# Decision: move the shell from WebKitGTK to Chromium

**Status: option 1 shipped in v0.4.0 (2026-07-15) and is still how Windows
and macOS open the window. On Linux, option 2 shipped on 2026-09-05: lich
bundles its own Chromium (CEF, through kurogane) and no browser is required —
see the section at the end.**

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

## Option 2 — embedded CEF (shipped on Linux, 2026-09-05)

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

Windows and macOS stay on option 1 for now. The window has only been built
and measured on Linux; macOS in particular needs the CEF framework plus its
helper apps laid out inside `Lich.app`, and there is no hardware here to
measure it on. `docs/ceilings.md` carries the gap.
