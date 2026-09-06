#!/bin/sh
# Assembles the window's runtime directory out of a cargo release build, the
# Windows layout: everything flat beside the executable, which is where CEF
# looks for libcef.dll on this platform (build/linux/shell/assemble.sh is the
# Linux counterpart, with the runtime under cef/).
#
#   <out>/lich-shell.exe   the binary (its icon and manifest compiled in)
#   <out>/*.dll ...        the CEF runtime, reduced to what lich needs
#
# usage: assemble.sh <cargo target/release dir> <out dir>
set -eu
src="$1"
out="$2"

rm -rf "$out"
mkdir -p "$out/locales"
cp "$src/lich-shell.exe" "$out/lich-shell.exe"

# The Windows distribution ships libcef.dll without symbols already, so
# nothing is stripped. d3dcompiler_47 is ANGLE's shader compiler on D3D11,
# the default GPU path; dxcompiler and dxil are Dawn's for WebGPU.
for f in libcef.dll chrome_elf.dll libEGL.dll libGLESv2.dll d3dcompiler_47.dll \
  dxcompiler.dll dxil.dll v8_context_snapshot.bin \
  chrome_100_percent.pak chrome_200_percent.pak resources.pak icudtl.dat; do
  cp "$src/$f" "$out/$f"
done
# The UI is English and a locale pack only translates Chromium's own dialogs;
# en-US is the one Chromium falls back to, and the one kurogane checks for.
cp "$src/locales/en-US.pak" "$out/locales/"
# Left out on purpose, as on Linux: vk_swiftshader.dll, vk_swiftshader_icd.json
# and vulkan-1.dll, the software WebGL for a machine with no GPU. xterm.js falls
# back to its canvas renderer there, at 6 MB less for everyone else.
