# The Linux window (`shell/`)

`shell/` is the window lich opens on Linux: an embedded Chromium (CEF), a thin
Rust crate over [kurogane](https://github.com/0x48piraj/kurogane). The Go
backend launches it as a subprocess with the `internal/chromium.Args` argv —
`--exit-on-stdin-eof` and the pipe behind it are what end the window with the
lich that opened it — so nothing about it reaches the pure-Go build.

## The kurogane pin

`shell/Cargo.toml` pins kurogane at the fork `omartelo/kurogane`, branch
`lich-close-prompt`: `lich-master` (upstream master plus Chromium's status
bubble kept off the window,
[0x48piraj/kurogane#23](https://github.com/0x48piraj/kurogane/pull/23)) and
`App::on_before_unload`, which lets lich-shell keep a close the user made so the
page can ask whether lich keeps running. Neither is upstream yet.
Everything else the fork once carried is upstream's own now, in upstream's
shape: the window's class, icon and title, the profile directory, where the
window closed and where it reopens, the decision hooks that grew out of
[#12](https://github.com/0x48piraj/kurogane/pull/12), the page-first key
([#21](https://github.com/0x48piraj/kurogane/pull/21)), a second launch
raising the window, the sandbox as a mode, the macOS menus and bundle layout.
When both land, point `shell/Cargo.toml` at `0x48piraj/kurogane`. Nothing
else changes.

## Building

kurogane links nothing of CEF: the window loads `libcef.so` when it starts,
from the runtime beside it, so building the crate is `cargo build` and the
CEF distribution is only what ships. `task build:shell` builds the crate,
fetches the distribution with `build/cef.sh` (tetsu's own `export-cef-dir`,
at the version the bindings were generated for, read off `Cargo.lock`) and
runs `assemble.sh`, which reduces the 1.4 GB distribution to what ships:

- `kurogane-bundle`, an empty marker beside the binary: it tells kurogane the
  runtime is in `cef/` and that it runs that one and no other, whatever a
  `CEF_PATH` in the environment says.
- `libcef.so` fully stripped: debug info and symbol table are ~1.1 GB of the
  1.4 GB, and what is left is ~260 MB. (A segfault was once blamed on the
  symbol table going; it was the feature-list overwrite `shell/src/main.rs`
  guards against, and a full strip was measured clean after that fix.)
- resources (`.pak`, `icudtl.dat`, the V8 snapshot) and only
  `locales/en-US.pak` — the UI is English and the other 219 packs only translate
  Chromium's own dialogs. ANGLE is inside `libcef.so` since CEF 154.
- SwiftShader and the Vulkan loader are left out: software WebGL for a GPU-less
  machine, where xterm.js falls back to its canvas renderer anyway. 16 MB saved.

On disk ~300 MB; ~100 MB compressed into the package. The first build fetches
the distribution (~200 MB download); later builds, in any worktree on the same CEF version, reuse the copy
`build/cef.sh --dir` prints.

## First build needs

A stable Rust toolchain and the libraries Chromium links against
(`libgtk-3-dev libnss3-dev libnspr4-dev libasound2-dev libcups2-dev libdrm-dev
libgbm-dev libxcomposite-dev libxdamage-dev libxfixes-dev libxrandr-dev
libxkbcommon-dev libxss-dev libxtst-dev libwayland-dev` on Debian/Ubuntu).
