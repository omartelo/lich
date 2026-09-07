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
[0x48piraj/kurogane#11](https://github.com/0x48piraj/kurogane/pull/11), with
the client handlers a browser delegate supplies (the keyboard handler that
hands the page Chromium's reserved chords) as
[0x48piraj/kurogane#12](https://github.com/0x48piraj/kurogane/pull/12), the
`NSApplication` kept to the browser process as
[0x48piraj/kurogane#14](https://github.com/0x48piraj/kurogane/pull/14), and
the macOS bundle layout (the framework resolved from `Contents/Frameworks`,
the subprocesses run as the bundle's helper app), which upstream carries on
its `seal-of-approval/distribution` branch
([53d51ba](https://github.com/0x48piraj/kurogane/commit/53d51ba7d234161f8709109dcb8d6395f38c7bb4));
lich's own PRs for that layout, #13 and #15, were closed as covered. One more
fix is the fork's alone so far: a second launch on the profile makes CEF
ask the running browser what to do with it, and kurogane answered nothing,
so CEF opened a Chrome-style browser no window of ours owned and the app
outlived its last window (lich#470); the fork raises the window it already
has. All of it is carried meanwhile on the fork `shell/Cargo.toml` pins:
`omartelo/kurogane`, branch `lich`, on top of upstream `eedaedc`.
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
