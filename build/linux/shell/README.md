# The Linux window (`shell/`)

`shell/` is the window lich opens on Linux: an embedded Chromium (CEF), a thin
Rust crate over [kurogane](https://github.com/0x48piraj/kurogane). The Go
backend launches it exactly the way it launches a system browser — same
`internal/chromium.Args` argv — so nothing about it reaches the pure-Go build.

## The kurogane fork

kurogane could not yet name the window it creates (WM_CLASS / Wayland app_id,
and a title) or place its Chromium profile where lich keeps one. Three builder
methods fix that — `App::window_class`, `App::window_title`, `App::cache_dir` —
submitted upstream as
[0x48piraj/kurogane#11](https://github.com/0x48piraj/kurogane/pull/11) and
carried meanwhile on the fork `shell/Cargo.toml` pins:
`omartelo/kurogane`, branch `window-identity`, on top of upstream `eedaedc`.
One wrinkle the patch works around: cef-rs hands CEF a *borrowed* string when
it writes an out-parameter struct back, so a `wm_class_class` built from `&str`
arrives empty — the fork allocates those through CEF's own
`cef_string_utf16_set` so they survive the write-back.

When the PR lands, point `shell/Cargo.toml` back at `0x48piraj/kurogane` at a
rev that includes it. Nothing else changes.

## Building

`task build:shell` compiles the crate and runs `assemble.sh`, which reduces the
1.3 GB cargo output to what ships:

- `libcef.so` **`--strip-debug` only** — a full `strip` removes the symbol table
  CEF resolves at runtime and the window segfaults on first paint (measured).
  Debug info is ~1 GB; the symbols are another ~180 MB and stay. Result ~430 MB.
- resources (`.pak`, `icudtl.dat`, the V8 snapshot), the ANGLE GL libs, and only
  `locales/en-US.pak` — the UI is English and the other 219 packs only translate
  Chromium's own dialogs.
- SwiftShader and the Vulkan loader are left out: software WebGL for a GPU-less
  machine, where xterm.js falls back to its canvas renderer anyway. 16 MB saved.

On disk ~480 MB; ~120 MB compressed into the package. The first build downloads
the CEF distribution (~200 MB) and compiles its C++ wrapper — CMake and Ninja
required, a few minutes once.

## First build needs

A stable Rust toolchain, CMake, Ninja, and the libraries Chromium links against
(`libgtk-3-dev libnss3-dev libnspr4-dev libasound2-dev libcups2-dev libdrm-dev
libgbm-dev libxcomposite-dev libxdamage-dev libxfixes-dev libxrandr-dev
libxkbcommon-dev libxss-dev libxtst-dev libwayland-dev` on Debian/Ubuntu).
