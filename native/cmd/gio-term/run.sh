#!/bin/sh
# Builds and runs the spike against a libghostty-vt built from ghostty main:
#   zig build -Demit-lib-vt -Doptimize=ReleaseFast --prefix "$GHOSTTY_VT"
# Scenarios:
#   ./run.sh                                   your shell
#   ./run.sh nvim +'call timer_start(16, {-> execute("normal! \<C-e>")}, {"repeat": -1})' big.go
#   ./run.sh sh -c 'cat big.log'               throughput
#   ./run.sh claude                            TUI streaming
set -eu
: "${GHOSTTY_VT:?set GHOSTTY_VT to the libghostty-vt install prefix}"
cd "$(dirname "$0")"
PKG_CONFIG_PATH="$GHOSTTY_VT/share/pkgconfig" go build -o gio-term .
exec ./gio-term "$@"
