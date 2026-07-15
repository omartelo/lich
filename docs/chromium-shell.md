# Decision: move the shell from WebKitGTK to Chromium

**Status: spike VALIDATED (2026-07-15) on the reference machine — "é outro
terminal", paint jank gone under Chromium with the identical terminal stack
and an uncoalesced transport. The migration below is greenlit; option 2 is
still deferred until the project grows.**

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
  New runtime requirement: chromium/chrome installed (fine for a personal
  harness on Arch; the launcher should probe `chromium`, `google-chrome`,
  `chromium-browser` and fail with a clear message).

Why lich is ~80% there already:

- Terminal I/O already rides a loopback WebSocket with token auth
  (`internal/terminal/transport.go` ↔ `frontend/src/lib/term-transport.ts`) —
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
4. **Packaging**: AppImage/deb/rpm ship only the Go binary; drop
   `fix-appimage.sh` and the bundled WebKitGTK entirely.
5. **Cleanup**: default the shell to Chromium, delete the Wails path, the
   ghostty patches and the GDK_BACKEND hack.

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

## Option 2 — embedded CEF via `energye/energy` (deferred)

Chromium compiled into/shipped with the app (CEF), Go bindings through the
`energy` framework. No dependency on a system browser; full control of the
Chromium version; window is truly ours.

Costs, and why it waits:

- +150-200MB bundle (CEF binaries per-arch), CI packaging gets heavy.
- `energy` is the only maintained Go/CEF route; ecosystem is thin and exotic
  compared to plain `net/http` + a browser flag.
- Every benefit it adds over option 1 only matters when lich stops being a
  personal harness — i.e. distribution to machines we don't control, where
  "install chromium" is unacceptable friction.

**Trigger to revisit**: the project grows an audience — packaging for users
who won't install a browser dependency, or a hard requirement to pin the
Chromium version. The migration path from option 1 is small: the whole app is
already "Go server + browser window"; option 2 only swaps who provides the
window.
