#!/bin/sh
# Assembles the window's runtime directory out of a cargo release build and a
# CEF distribution (build/cef.sh fetches one), the macOS layout: the binary
# beside the CEF framework, which is what build/darwin/bundle.sh lays into
# Lich.app (build/linux/shell/assemble.sh and build/windows/shell/assemble.sh
# are the counterparts).
#
#   <out>/lich-shell                              the binary
#   <out>/Chromium Embedded Framework.framework   the CEF runtime, reduced to what lich needs
#
# No bundle marker: inside Lich.app kurogane reads the framework from
# Contents/Frameworks by the bundle's own layout, and outside one it finds the
# framework beside the executable.
#
# usage: assemble.sh <cargo target/release dir> <CEF distribution dir> <out dir>
set -eu
src="$1"
cef="$2"
out="$3"
name="Chromium Embedded Framework.framework"

framework="$(find "$cef" -maxdepth 3 -type d -name "$name" | head -n 1)"
test -n "$framework" || { echo "assemble.sh: no $name under $cef" >&2; exit 1; }

rm -rf "$out"
mkdir -p "$out"
cp "$src/lich-shell" "$out/lich-shell"
# ditto, not cp: it keeps the framework's symlinks and permissions as they are.
ditto "$framework" "$out/$name"

fw="$out/$name"
# The UI is English and a locale pack only translates Chromium's own dialogs;
# en is the one Chromium falls back to, and kurogane checks for any .lproj.
test -d "$fw/Resources/en.lproj" || { echo "assemble.sh: no en.lproj in $fw/Resources" >&2; exit 1; }
find "$fw/Resources" -maxdepth 1 -name '*.lproj' ! -name 'en.lproj' -exec rm -r {} +
# Left out on purpose, as on Linux and Windows: SwiftShader and the Vulkan
# loader, the software WebGL for a machine with no GPU. xterm.js falls back to
# its canvas renderer. ANGLE is inside the framework's binary since CEF 154.
rm -f "$fw/Libraries/libvk_swiftshader.dylib" "$fw/Libraries/vk_swiftshader_icd.json" \
  "$fw/Libraries/libvulkan.dylib"
