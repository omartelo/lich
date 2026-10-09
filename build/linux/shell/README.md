# The Linux window (`shell/`)

`shell/` is the window lich opens on Linux: an embedded Chromium (CEF), a thin
Rust crate over [kurogane](https://github.com/0x48piraj/kurogane). The Go
backend launches it as a subprocess with the `internal/chromium.Args` argv —
`--exit-on-stdin-eof` and the pipe behind it are what end the window with the
lich that opened it — so nothing about it reaches the pure-Go build.

## The kurogane fork

kurogane could not yet name the window it creates (WM_CLASS / Wayland app_id,
a title, an icon) or place its Chromium profile where lich keeps one. Four
builder methods fix that — `App::window_class`, `App::window_title`,
`App::window_icon`, `App::cache_dir` — submitted upstream as
[0x48piraj/kurogane#11](https://github.com/0x48piraj/kurogane/pull/11). The
main window opened at CEF's default size every launch: the fork asks the
browser delegate for its initial geometry and tells it the bounds the window
closed with
([0x48piraj/kurogane#19](https://github.com/0x48piraj/kurogane/pull/19)), and
`shell/src/geometry.rs` remembers them. And the key hook could let a key go on
or take it, never hand it to the page ahead of Chromium's reserved
accelerators, which is the one thing this window asks of it:
`KeyDecision::PageFirst`
([0x48piraj/kurogane#21](https://github.com/0x48piraj/kurogane/pull/21)).
The main window also creates its browser with Chromium's status bubble off,
which otherwise shows a hovered link's URL, token included, in the window's
corner (`e58b415`, not submitted upstream yet).

The rest of what the fork once carried is upstream's now, in upstream's own
shape: the decision hooks that grew out of
[#12](https://github.com/0x48piraj/kurogane/pull/12), a second launch on the
profile raising the window it already has (lich#470), the sandbox as a mode
the host picks, which `shell/src/main.rs` sets to Chromium's on a Linux
machine that can confine its subprocesses and leaves off elsewhere, the macOS
application and Edit menus, the `NSApplication` kept to the browser process
(#14) and the macOS bundle layout.

All of it is pinned on the fork: `omartelo/kurogane`, branch `lich-hooks`, on
top of upstream's `native-tongue/capabilities` at `ba69cfd`, the branch the
hooks live on until it reaches master.
One wrinkle the patch works around: cef-rs hands CEF a *borrowed* string when
it writes an out-parameter struct back, so a `wm_class_class` built from `&str`
arrives empty — the fork allocates those through CEF's own
`cef_string_utf16_set` so they survive the write-back.

When the PRs land, point `shell/Cargo.toml` back at `0x48piraj/kurogane` at
a rev that includes them all. Nothing else changes.

## Building

`task build:shell` compiles the crate and runs `assemble.sh`, which reduces the
1.3 GB cargo output to what ships:

- `libcef.so` fully stripped: debug info and symbol table are ~1.1 GB of the
  1.3 GB, and what is left is ~260 MB. (A segfault was once blamed on the
  symbol table going; it was the feature-list overwrite `shell/src/main.rs`
  guards against, and a full strip was measured clean after that fix.)
- resources (`.pak`, `icudtl.dat`, the V8 snapshot), the ANGLE GL libs, and only
  `locales/en-US.pak` — the UI is English and the other 219 packs only translate
  Chromium's own dialogs.
- SwiftShader and the Vulkan loader are left out: software WebGL for a GPU-less
  machine, where xterm.js falls back to its canvas renderer anyway. 16 MB saved.

On disk ~300 MB; ~100 MB compressed into the package. The first build downloads
the CEF distribution (~200 MB) and compiles its C++ wrapper — CMake and Ninja
required, a few minutes once.

## First build needs

A stable Rust toolchain, CMake, Ninja, and the libraries Chromium links against
(`libgtk-3-dev libnss3-dev libnspr4-dev libasound2-dev libcups2-dev libdrm-dev
libgbm-dev libxcomposite-dev libxdamage-dev libxfixes-dev libxrandr-dev
libxkbcommon-dev libxss-dev libxtst-dev libwayland-dev` on Debian/Ubuntu).
