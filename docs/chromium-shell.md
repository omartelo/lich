# Decision: move the shell from WebKitGTK to Chromium

**Status: direction decided (2026-07-15) — option 1 first; option 2 revisited
if the project grows. Not started; the spike is the next step.**

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

Remaining work (the actual migration):

1. **RPC-ify the Wails bindings** (~20 calls: terminal `Start/Write/Resize/
   SetVisible/Close`, projects/sessions CRUD, git status, plugin gate...) onto
   the existing WS (or plain HTTP on the same listener). The Wails
   `Events.On` fallback path dies; the WS becomes the only channel.
2. **Native folder picker** → `github.com/ncruces/zenity` (zenity /
   xdg-desktop-portal under the hood).
3. **Clipboard** → `navigator.clipboard` (localhost is a secure context in
   Chromium); the Wails clipboard binding dies.
4. **Lifecycle**: launch, crash/restart of the browser process, single-
   instance lock, `--class`/icon for the WM.
5. **Packaging**: AppImage/deb/rpm keep shipping only the Go binary; drop
   `fix-appimage.sh` and the bundled WebKitGTK entirely.

What dies with WebKitGTK (all "Known Ceilings" entries): forced
`GDK_BACKEND=x11`, the sandbox-disabled AppImage, the contenteditable DOM
guard, middle-click-paste quirks — plus, if the xterm migration is confirmed
by the same spike, the entire ghostty-web private-patching layer
(`render-pause`, `glyph-atlas`, `row-paint`, `scrollback-perf`, `getline-pool`,
`block-glyphs`, `font-metrics`, the 0.4.0 pin).

Trade-offs accepted: the window belongs to Chromium (no native menus — unused
anyway; icon/class via flags), and the Chromium version tracks the system.

### Spike (next step, ~1 day)

Serve `frontend/dist` over loopback, launch `chromium --app`, terminal via the
existing WS (xterm.js + WebGL, POC component), everything else stubbed.
Measure the same scenarios (nvim scroll, Claude Code streaming) with
`term-perf`. Jank gone → commit to the migration above.

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
