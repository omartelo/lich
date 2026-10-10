#!/bin/bash
# Runs a freshly bundled Lich.app end to end on the macOS runner, the Windows
# job's check on the bundle: the seal verifies, lich finds the window beside
# itself in Contents/MacOS and opens it, the page loads (over CDP: alive alone
# is not proof), macOS counts exactly one application in Lich.app (lsappinfo,
# which is what the Dock reads: a second entry is a subprocess with an
# NSApplication of its own, and a Dock tile of its own with it, four of them
# before kurogane#14), a second launch is forwarded to the window without
# opening a second browser in it (#470), the window closes on SIGTERM, which
# is what quitting and the restart flow send, lich keeps running without it,
# a launch opens a new window on it, and `lich quit` ends it. Its config lives
# under a HOME of its own.
# What a failure leaves behind goes to $RUNNER_TEMP/diag for the artifact the
# workflow uploads.
#
# usage: mac-e2e.sh <path to Lich.app>
set -eu

app="$(cd "$1" && pwd)"
export HOME="$RUNNER_TEMP/home" LICH_LISTEN_PORT=47899
config="$HOME/Library/Application Support/lich"
log="$config/lich.log"
diag="$RUNNER_TEMP/diag"
pages="$RUNNER_TEMP/pages.json"
mkdir -p "$HOME" "$diag"

# The helper apps are "Lich Helper (Renderer)" and so on: a case-insensitive
# match is what catches them beside lich and lich-shell.
dump() {
  cp "$pages" "$diag/" 2>/dev/null || true
  ps -eo pid,ppid,args | grep -i '[l]ich' > "$diag/ps.txt" || true
  lsappinfo list > "$diag/apps.txt" || true
  screencapture -x "$diag/screen.png" || true
  cp "$log" "$diag/" 2>/dev/null || true
  # A subprocess that died leaves its report with the system, keyed by the
  # user, not by HOME.
  cp /Users/runner/Library/Logs/DiagnosticReports/[lL]ich* "$diag/" 2>/dev/null || true
  cat "$diag/ps.txt"
}

fail() {
  echo "::error::$1"
  dump
  cat "$log" 2>/dev/null || true
  exit 1
}

codesign --verify --deep --strict --verbose=2 "$app"
test -f "$app/Contents/MacOS/lich-shell"
test -f "$app/Contents/Frameworks/Chromium Embedded Framework.framework/Chromium Embedded Framework"

# Chromium's own log on stderr: a renderer or GPU process that fails to launch
# says so there and nowhere else.
"$app/Contents/MacOS/lich" -- --remote-debugging-port=9334 --enable-logging=stderr &
token=""
for _ in $(seq 1 30); do
  sleep 1
  token=$(jq -r .token "$config/runtime.json" 2>/dev/null) && break
done
test -n "$token" || fail "lich never wrote runtime.json"
grep -q 'the bundled window' "$log" || fail "lich did not resolve its own window"

for _ in $(seq 1 30); do
  sleep 1
  # Every CEF role alive while waiting: a renderer that comes and goes shows
  # up here and nowhere else.
  ps -eo args | grep -i 'lich' | grep -o -- '--type=[a-z-]*' | sort -u >> "$diag/roles.txt" || true
  curl -sf http://127.0.0.1:9334/json > "$pages" 2>/dev/null && grep -q '"title": *"lich"' "$pages" && break
done
grep -q '"title": *"lich"' "$pages" || { sort -u "$diag/roles.txt"; fail "the window never loaded lich's page"; }
sleep 10

# The browser process runs as lich-shell; the subprocesses are the helper apps.
browser=$(pgrep -x lich-shell || true)
test -n "$browser" || fail "the window died"

# A second launch: lich finds the first on its port and asks it for its window;
# with one open, that lich hands it its URL again, which CEF forwards to the
# running browser. The forward used to open a
# second browser in it, a page more on CDP and a process that outlived the
# window's close (#470); the window now raises itself and the count stays one.
"$app/Contents/MacOS/lich" || fail "the duplicate launch exited $?"
sleep 5
curl -sf http://127.0.0.1:9334/json > "$pages"
count=$(grep -c '"type": *"page"' "$pages" || true)
[ "$count" = 1 ] || fail "the forwarded launch left $count pages in the window, expected one"

lsappinfo list > "$RUNNER_TEMP/apps.txt"
apps=$(grep -c 'bundle path=.*Lich.app' "$RUNNER_TEMP/apps.txt" || true)
[ "$apps" = 1 ] || { grep -B1 -A3 'Lich.app' "$RUNNER_TEMP/apps.txt"; fail "macOS counts $apps applications in Lich.app, expected the window alone"; }

# Closing the window leaves lich running with its sessions; with none running
# it would quit, so one is opened first, through the CLI as an agent would.
mkdir -p "$RUNNER_TEMP/proj"
"$app/Contents/MacOS/lich" open --project "$RUNNER_TEMP/proj" --kind shell || fail "lich open exited $?"
kill -TERM "$browser"
for _ in $(seq 1 15); do sleep 1; pgrep -x lich-shell > /dev/null || break; done
pgrep -x lich-shell > /dev/null && fail "the window outlived its SIGTERM"
sleep 2
pgrep -x lich > /dev/null || fail "lich exited with its window"
grep -q 'window closed, lich keeps running' "$log" || fail "lich did not log the window closing"

# Launched again, lich asks the running one for its window, and that backend
# opens it: the same extra switches, so the page is on CDP again.
"$app/Contents/MacOS/lich" || fail "the reopening launch exited $?"
for _ in $(seq 1 30); do
  sleep 1
  curl -sf http://127.0.0.1:9334/json > "$pages" 2>/dev/null && grep -q '"title": *"lich"' "$pages" && break
done
grep -q '"title": *"lich"' "$pages" || fail "a second launch did not reopen the window on the running lich"

"$app/Contents/MacOS/lich" quit || fail "lich quit exited $?"
for _ in $(seq 1 10); do pgrep -x lich > /dev/null || break; sleep 1; done
pgrep -x lich > /dev/null && fail "lich is still running after lich quit"
pgrep -x lich-shell > /dev/null && fail "the window outlived lich quit"
cat "$pages"
grep 'Lich.app' "$RUNNER_TEMP/apps.txt"
