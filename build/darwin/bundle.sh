#!/bin/sh
# Wraps an already-built darwin binary in Lich.app — the bundle is what puts
# lich in /Applications, Launchpad and Spotlight and gives it its icon; a bare
# binary on PATH appears in none of them.
#
#   build/darwin/bundle.sh bin/lich-darwin-arm64 0.30.0 bin
#
# macOS host only: sips and iconutil are Apple's, so the icon is generated at
# build time from build/appicon.png instead of committing an .icns — one source
# of the mark for every platform. Cross-compiling the binary still works
# anywhere (task build:mac); only this wrapping step needs a Mac.
set -eu

bin="${1:?usage: bundle.sh <binary> <version> <outdir>}"
version="${2:?usage: bundle.sh <binary> <version> <outdir>}"
outdir="${3:?usage: bundle.sh <binary> <version> <outdir>}"

root="$(cd "$(dirname "$0")/../.." && pwd)"
app="$outdir/Lich.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/lich"
chmod +x "$app/Contents/MacOS/lich"

sed "s/@VERSION@/${version}/g" "$root/build/darwin/Info.plist.tpl" \
  > "$app/Contents/Info.plist"

iconset="$(mktemp -d)/lich.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$root/build/appicon.png" \
    --out "$iconset/icon_${size}x${size}.png" >/dev/null
  sips -z "$((size * 2))" "$((size * 2))" "$root/build/appicon.png" \
    --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/lich.icns"
rm -rf "$(dirname "$iconset")"

# Ad-hoc signature: arm64 refuses to run an executable carrying none, and a
# bundle whose seal does not cover Info.plist and the icon reads as damaged.
# This is not notarization — Gatekeeper still calls the developer unidentified.
codesign --force --sign - "$app"

echo "built $app"
