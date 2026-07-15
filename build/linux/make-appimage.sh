#!/usr/bin/env bash
# Wraps the static lich binary in an AppImage. There is nothing to bundle —
# no GTK, no WebKit (docs/chromium-shell.md phase 5) — so the AppDir is just
# the binary, the .desktop file and the icon, packed with appimagetool.
set -euo pipefail

BINARY="${1:?usage: make-appimage.sh <binary> <output-dir>}"
OUTPUT_DIR="${2:?usage: make-appimage.sh <binary> <output-dir>}"
ARCH="$(uname -m)"
HERE="$(cd "$(dirname "$0")" && pwd)"
APPDIR="$(mktemp -d)/lich.AppDir"

command -v appimagetool >/dev/null || {
  echo "appimagetool is required (https://github.com/AppImage/appimagetool/releases)" >&2
  exit 1
}

mkdir -p "$APPDIR"
cp "$BINARY" "$APPDIR/lich"
cp "$HERE/lich.desktop" "$APPDIR/lich.desktop"
cp "$HERE/../appicon.png" "$APPDIR/lich.png"
cp "$HERE/../appicon.png" "$APPDIR/.DirIcon"
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
exec "$(dirname "$0")/lich" "$@"
EOF
chmod +x "$APPDIR/AppRun" "$APPDIR/lich"

appimagetool "$APPDIR" "$OUTPUT_DIR/lich-${ARCH}.AppImage"
rm -rf "$(dirname "$APPDIR")"
echo "built $OUTPUT_DIR/lich-${ARCH}.AppImage"
